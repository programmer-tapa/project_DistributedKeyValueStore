package dkv;

import java.io.BufferedReader;
import java.io.InputStreamReader;
import java.util.ArrayList;
import java.util.List;

public class DKVClient {
    private final String ctrlerAddr;
    private final String clientBin;

    /**
     * Initializes the DKV client wrapper with defaults (localhost:9000, dkv-client).
     */
    public DKVClient() {
        this("localhost:9000", "dkv-client");
    }

    /**
     * Initializes the DKV client wrapper.
     * 
     * @param ctrlerAddr Address of the Shard Controller or config store.
     * @param clientBin Path to the compiled dkv-client CLI binary.
     */
    public final void dkvInit(String ctrlerAddr, String clientBin) {
        // Method helper to re-initialize if needed.
    }

    public DKVClient(String ctrlerAddr, String clientBin) {
        this.ctrlerAddr = ctrlerAddr;
        this.clientBin = clientBin;
    }

    public static class GetResult {
        public final String value;
        public final long version;

        public GetResult(String value, long version) {
            this.value = value;
            this.version = version;
        }
    }

    /**
     * Writes a key-value pair to the distributed key-value store.
     * 
     * @param key Key to write.
     * @param value Value to write.
     * @return true if successful, throws Exception on failure.
     */
    public boolean put(String key, String value) throws Exception {
        return put(key, value, null);
    }

    /**
     * Writes a key-value pair to the distributed key-value store with a version constraint.
     * 
     * @param key Key to write.
     * @param value Value to write.
     * @param version Optional version constraint.
     * @return true if successful, throws Exception on failure.
     */
    public boolean put(String key, String value, Long version) throws Exception {
        List<String> command = new ArrayList<>();
        command.add(clientBin);
        command.add("--ctrler-addr");
        command.add(ctrlerAddr);
        command.add("put");
        command.add(key);
        command.add(value);
        if (version != null) {
            command.add(version.toString());
        }

        ProcessBuilder pb = new ProcessBuilder(command);
        Process process = pb.start();

        int exitCode = process.waitFor();
        if (exitCode != 0) {
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getErrorStream()))) {
                StringBuilder sb = new StringBuilder();
                String line;
                while ((line = reader.readLine()) != null) {
                    sb.append(line).append("\n");
                }
                throw new Exception("DKV Put failed: " + sb.toString().trim());
            }
        }

        try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()))) {
            String line = reader.readLine();
            return line != null && line.trim().equals("OK");
        }
    }

    /**
     * Reads a key's value and version from the distributed store.
     * 
     * @param key Key to read.
     * @return GetResult containing value and version. Returns null value and version 0 if key not found.
     */
    public GetResult get(String key) throws Exception {
        List<String> command = new ArrayList<>();
        command.add(clientBin);
        command.add("--ctrler-addr");
        command.add(ctrlerAddr);
        command.add("get");
        command.add(key);

        ProcessBuilder pb = new ProcessBuilder(command);
        Process process = pb.start();

        int exitCode = process.waitFor();
        if (exitCode != 0) {
            try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getErrorStream()))) {
                StringBuilder sb = new StringBuilder();
                String line;
                while ((line = reader.readLine()) != null) {
                    sb.append(line).append("\n");
                }
                String err = sb.toString().trim();
                if (err.contains("Key not found") || exitCode == 1) {
                    return new GetResult(null, 0);
                }
                throw new Exception("DKV Get failed: " + err);
            }
        }

        try (BufferedReader reader = new BufferedReader(new InputStreamReader(process.getInputStream()))) {
            String line = reader.readLine();
            if (line != null && line.trim().startsWith("Value: ")) {
                String valPart = line.trim().substring("Value: ".length());
                String[] parts = valPart.split(" \\(Version: ");
                String val = parts[0];
                long ver = Long.parseLong(parts[1].substring(0, parts[1].length() - 1));
                return new GetResult(val, ver);
            }
        }
        return new GetResult(null, 0);
    }
}
