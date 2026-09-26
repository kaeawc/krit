// Compiler-test source stub; never packaged in the production artifact.
package android.graphics;

import android.content.res.Resources;
import java.io.FileDescriptor;
import java.io.InputStream;

// decodeResourceStream (which takes android.util.TypedValue) and the
// ColorSpace-typed Options fields are not modeled.
public class BitmapFactory {
    public BitmapFactory() {
    }

    public static Bitmap decodeFile(String pathName, Options opts) {
        throw new RuntimeException("Stub!");
    }

    public static Bitmap decodeFile(String pathName) {
        throw new RuntimeException("Stub!");
    }

    public static Bitmap decodeResource(Resources res, int id, Options opts) {
        throw new RuntimeException("Stub!");
    }

    public static Bitmap decodeResource(Resources res, int id) {
        throw new RuntimeException("Stub!");
    }

    public static Bitmap decodeByteArray(byte[] data, int offset, int length, Options opts) {
        throw new RuntimeException("Stub!");
    }

    public static Bitmap decodeByteArray(byte[] data, int offset, int length) {
        throw new RuntimeException("Stub!");
    }

    public static Bitmap decodeStream(InputStream is, Rect outPadding, Options opts) {
        throw new RuntimeException("Stub!");
    }

    public static Bitmap decodeStream(InputStream is) {
        throw new RuntimeException("Stub!");
    }

    public static Bitmap decodeFileDescriptor(FileDescriptor fd, Rect outPadding, Options opts) {
        throw new RuntimeException("Stub!");
    }

    public static Bitmap decodeFileDescriptor(FileDescriptor fd) {
        throw new RuntimeException("Stub!");
    }

    public static class Options {
        public Bitmap inBitmap;

        public boolean inMutable;

        public boolean inJustDecodeBounds;

        public int inSampleSize;

        public Bitmap.Config inPreferredConfig = Bitmap.Config.ARGB_8888;

        public boolean inPremultiplied;

        public int inDensity;

        public int inTargetDensity;

        public int inScreenDensity;

        public boolean inScaled;

        @Deprecated
        public boolean inPurgeable;

        @Deprecated
        public boolean inInputShareable;

        public int outWidth;

        public int outHeight;

        public String outMimeType;

        public Bitmap.Config outConfig;

        public byte[] inTempStorage;

        public Options() {
        }
    }
}
