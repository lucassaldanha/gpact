package net.consensys.gpact.examples.sfc.write;

import net.consensys.gpact.helpers.AbstractExampleTest;
import org.junit.jupiter.api.Test;

public class SfcCrosschainWriteTest extends AbstractExampleTest {

  @Test
  public void directSignMultipleBlockchain() throws Exception {
    String tempPropsFile = createPropertiesFile(true, false, false);
    SfcCrosschainWrite.main(new String[] {tempPropsFile});
  }
}