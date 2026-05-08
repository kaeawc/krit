package test;

import okhttp3.CertificatePinner;

class EmptyPinner {
    void pinner() {
        new CertificatePinner.Builder().build();
    }
}
