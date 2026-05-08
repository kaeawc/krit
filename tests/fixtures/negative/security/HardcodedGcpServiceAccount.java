package test;

import java.nio.file.Files;
import java.nio.file.Path;

class Credentials {
  String load() throws Exception {
    return Files.readString(Path.of("service-account.json"));
  }
}
