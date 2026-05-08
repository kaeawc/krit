package test;

class AuthClient {
  void send(Request request) {
    request.header("Authorization", "Bearer sk_live_abcdef0123456789");
  }
}

interface Request {
  void header(String name, String value);
}
