package api

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/consensys/gpact/messaging/message-store/internal/logging"
	v1 "github.com/consensys/gpact/services/relayer/pkg/messages/v1"
	"github.com/gin-gonic/gin"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-ds-badger2"
	"net/http"
	"regexp"
)

type MessageStoreApi struct {
	DataStore *badger.Datastore
	idRegex   *regexp.Regexp
}

var MessageDetailsMismatchError = fmt.Errorf(
	"the details of the message submitted for update does not match those stored in the data store")

const MessageIdPattern = "\\w+-\\w{40,42}-\\d+-\\d+-\\d+"

// UpsertMessageHandler is a handler for PUT /messages and PUT /messages:id endpoints.
// The method adds a new message to a datastore, if it does not already exist.
// If the message already exists, it updates the proof set of the message in the datastore
// to include new proof elements from the message submitted.
// Parameters:
// - Path (optional): Message ID
// Responses:
// - HTTP 201: New message added to datastore
// - HTTP 200: Existing message updated
// - HTTP 400: Invalid parameters or payload submitted by client
func (mApi *MessageStoreApi) UpsertMessageHandler(c *gin.Context) {
	var message *v1.Message
	err := c.BindJSON(&message)
	if err != nil {
		return
	}

	if !mApi.isValidId(message.ID) {
		statusBadRequest(c, fmt.Sprintf("Message id '%s' is not valid", message.ID))
		return
	}

	paramId := c.Param("id")
	if len(paramId) > 0 && paramId != message.ID {
		statusBadRequest(c, fmt.Sprintf("Message id provided in the path parameter, '%s', "+
			"does not match id in the message body, '%s'", paramId, message.ID))
		return
	}

	tx, err := mApi.DataStore.NewTransaction(c, false)
	if err != nil {
		logging.Error("Error creating a transaction to update message %s: %v", message.ID, err)
		statusServerError(c, err)
		tx.Discard(c)
		return
	}

	created, err := mApi.upsertMessage(c, tx, message)
	if err == MessageDetailsMismatchError {
		statusBadRequest(c, err.Error())
	} else if err != nil {
		logging.Error("Error adding or updating message %s: %v", message.ID, err)
		statusServerError(c, err)
		tx.Discard(c)
		return
	}

	err = tx.Commit(c)
	if err != nil {
		logging.Error("Error committing updates to message %s: %v", message.ID, err)
		statusServerError(c, err)
		return
	}

	if created {
		statusCreated(c, "Message successfully added to the data store")
	} else {
		statusOk(c, "Message successfully updated")
	}
}

// RecordProofsHandler The endpoint adds the given proof elements to the set of proofs that has already been recorded
// for a message. If specific elements within the array of proof submitted have already been stored,
// they are ignored,  if not, they are added to the datastore.
// Parameters:
// - Path: Message ID
// - Body: Array of Proof elements
// Responses:
// - HTTP 201: One or more new proof elements in the request body, were added to the existing proof set for a message.
// - HTTP 200: All proof elements in the request body were already a part of the recorded proof set for a message,
//   		   so no updates needed to be performed.
// - HTTP 400: Invalid parameters or payload submitted by client
func (mApi *MessageStoreApi) RecordProofsHandler(c *gin.Context) {
	var newProofs []v1.Proof
	err := c.BindJSON(&newProofs)
	if err != nil {
		return
	}

	paramId := c.Param("id")
	if !mApi.isValidId(paramId) {
		statusBadRequest(c, fmt.Sprintf("Message id '%s' is not valid", paramId))
		return
	}

	if !mApi.messageExists(c, paramId) {
		statusMessageNotFound(c, paramId)
		return
	}

	tx, err := mApi.DataStore.NewTransaction(c, false)
	if err != nil {
		logging.Error("Error creating a transaction to update message %s: %v", paramId, err)
		statusServerError(c, err)
		tx.Discard(c)
		return
	}

	existingMsg, err := mApi.queryMessageById(c, datastore.NewKey(paramId), mApi.DataStore.Get)
	if err == datastore.ErrNotFound {
		statusMessageNotFound(c, paramId)
		return
	} else if err != nil {
		logging.Error("Error retrieving message %s: %v", paramId, err)
		statusServerError(c, err)
		return
	}

	updated, err := mApi.updateMessageProofSet(c, tx, existingMsg, newProofs)
	if err != nil {
		logging.Error("Error updating proof set for message %s: %v", paramId, err)
		statusServerError(c, err)
		tx.Discard(c)
		return
	}

	err = tx.Commit(c)
	if err != nil {
		logging.Error("Error committing updates to message %s: %v", paramId, err)
		statusServerError(c, err)
		return
	}

	if updated {
		statusCreated(c, "One or more proof elements submitted were successfully added to message's proof set")
	} else {
		statusOk(c, "All proof elements submitted are already a part of the message's proof set")
	}
}

// GetMessageHandler retrieves a message with the given ID if it exists in the datastore.
// Parameters:
//  - Path: Message ID
// Response Status:
// - HTTP 200: Message successfully retrieved
// - HTTP 400: Invalid message ID provided
// - HTTP 404: Message could not be found
func (mApi *MessageStoreApi) GetMessageHandler(c *gin.Context) {
	id := c.Param("id")
	if !mApi.isValidId(id) {
		statusBadRequest(c, fmt.Sprintf("message id '%s' is not valid", id))
		return
	}
	mApi.respondWithMessageDetails(c, id, nil)
}

// GetMessageProofsHandler retrieves the proof set for a message  with the given ID if it exists in the datastore.
// Parameters:
//  - Path: Message ID
// Response Status:
// - HTTP 200: Proof set for message successfully retrieved
// - HTTP 400: Invalid message ID provided
// - HTTP 404: Message could not be found
func (mApi *MessageStoreApi) GetMessageProofsHandler(c *gin.Context) {
	id := c.Param("id")
	if !mApi.isValidId(id) {
		statusBadRequest(c, fmt.Sprintf("Message id '%s' is not valid", id))
		return
	}
	mApi.respondWithMessageDetails(c, id, func(message *v1.Message) interface{} { return message.Proofs })
}

func (mApi *MessageStoreApi) respondWithMessageDetails(c *gin.Context, id string,
	attribExtractor func(message *v1.Message) interface{}) {
	message, err := mApi.queryMessageById(c, datastore.NewKey(id), mApi.DataStore.Get)
	if err == datastore.ErrNotFound {
		statusMessageNotFound(c, id)
		return
	} else if err != nil {
		logging.Error("Error retrieving message %s: %v", id, err)
		statusServerError(c, err)
		return
	}

	var msgDetails interface{} = message
	if attribExtractor != nil {
		msgDetails = attribExtractor(message)
	}

	c.JSON(http.StatusOK, msgDetails)
}

func (mApi *MessageStoreApi) isValidId(id string) bool {
	if mApi.idRegex == nil {
		r, err := regexp.Compile(MessageIdPattern)
		if err != nil {
			logging.Error("error compiling message Id regex. error: %v", err)
			return false
		}
		mApi.idRegex = r
	}
	return mApi.idRegex.MatchString(id)
}

// queryMessageById queries the datastore for a message with the given ID
func (mApi *MessageStoreApi) queryMessageById(c *gin.Context, id datastore.Key, dsQueryFunc func(c context.Context,
	key datastore.Key) ([]byte, error)) (*v1.Message, error) {
	msgBytes, err := dsQueryFunc(c, id)
	if err != nil {
		return nil, err
	}

	var msg v1.Message
	err = json.Unmarshal(msgBytes, &msg)
	if err != nil {
		return nil, err
	}

	return &msg, err
}

func (mApi *MessageStoreApi) messageExists(c *gin.Context, id string) bool {
	has, err := mApi.DataStore.Has(c, datastore.NewKey(id))
	if err != nil {
		return false
	}
	return has
}

// upsertMessage create or updates a message.
// Adds a message to the datastore if it does not already exist.
// If it exists, the proof set of the message in the datastore is updated
// with unique proof elements from the new message submitted.
func (mApi *MessageStoreApi) upsertMessage(c *gin.Context, tx datastore.Txn, newMessage *v1.Message) (bool, error) {
	id := datastore.NewKey(newMessage.ID)
	exists, err := tx.Has(c, id)
	if err != nil {
		return false, err
	}
	if exists {
		existingMsg, err := mApi.queryMessageById(c, id, tx.Get)
		if err != nil {
			return false, err
		}
		if !areImmutableDetailsSame(existingMsg, newMessage) {
			return false, MessageDetailsMismatchError
		}
		// TODO: validate that all other fields of the two messages match as a sanity check
		_, err = mApi.updateMessageProofSet(c, tx, existingMsg, newMessage.Proofs)
		return false, err
	} else {
		err = mApi.addMessage(c, tx, id, newMessage)
		return true, err
	}
}

// updateMessageProofSet adds new proof elements of the message provided as argument,
// to the proof set of the message in the datastore.
func (mApi *MessageStoreApi) updateMessageProofSet(c *gin.Context, tx datastore.Txn, existingMsg *v1.Message,
	newProofSet []v1.Proof) (bool, error) {
	oldProofCount := len(existingMsg.Proofs)
	existingMsg.Proofs = aggregateProofSets(existingMsg.Proofs, newProofSet)
	updatedMsg, err := json.Marshal(existingMsg)
	if err != nil {
		return false, err
	}

	err = tx.Put(c, datastore.NewKey(existingMsg.ID), updatedMsg)
	if err != nil {
		return false, err
	}
	newProofsFound := oldProofCount < len(existingMsg.Proofs)
	return newProofsFound, nil
}

func (mApi *MessageStoreApi) addMessage(c *gin.Context, tx datastore.Txn, msgId datastore.Key,
	newMessage *v1.Message) error {
	m, err := json.Marshal(newMessage)
	if err != nil {
		return err
	}
	return tx.Put(c, msgId, m)
}

// aggregateProofSets returns a proof set that combines unique elements from both proof arrays.
func aggregateProofSets(proofSetA []v1.Proof, proofSetB []v1.Proof) []v1.Proof {
	var combined = proofSetA
	// TODO: consider optimising
	for _, v := range proofSetB {
		// avoid potential duplicate proofs in the new set,
		// by checking the already updated proof set
		if !containsProof(combined, v) {
			combined = append(combined, v)
		}
	}
	return combined
}

func containsProof(proofSet []v1.Proof, proof v1.Proof) bool {
	for _, p := range proofSet {
		if p == proof {
			return true
		}
	}
	return false
}

// areImmutableDetailsSame checks that all details of two messages match, except for their proof payloads
func areImmutableDetailsSame(msg1 *v1.Message, msg2 *v1.Message) bool {
	return msg1.ID == msg2.ID &&
		msg1.MsgType == msg2.MsgType &&
		msg1.Timestamp == msg2.Timestamp &&
		msg1.Version == msg2.Version &&
		msg1.Source == msg2.Source &&
		msg1.Destination == msg2.Destination &&
		msg1.Payload == msg2.Payload
}

func NewMessageStoreService(dsPath string, options *badger.Options) (*MessageStoreApi, error) {
	ds, err := badger.NewDatastore(dsPath, options)
	if err != nil {
		return nil, err
	}
	return &MessageStoreApi{DataStore: ds}, nil
}
