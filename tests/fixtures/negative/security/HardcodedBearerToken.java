package test;

class AuthClient {
  void send(Request request) {
    request.header("Authorization", "Bearer " + BuildConfig.API_TOKEN);
    request.header("Authorization", "Bearer your_api_token_here");
  }
}

class BuildConfig {
  static final String API_TOKEN = "sk_live_abcdef0123456789";
}

interface Request {
  void header(String name, String value);
}
