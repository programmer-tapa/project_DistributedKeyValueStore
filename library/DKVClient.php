<?php

class DKVClient {
    private string $ctrlerAddr;
    private string $clientBin;

    /**
     * Initializes the DKV client wrapper.
     * 
     * @param string $ctrlerAddr Address of the Shard Controller or config store.
     * @param string $clientBin Path to the compiled dkv-client CLI binary.
     */
    public function __construct(string $ctrlerAddr = 'localhost:9000', string $clientBin = 'dkv-client') {
        $this->ctrlerAddr = $ctrlerAddr;
        $this->clientBin = $clientBin;
    }

    /**
     * Writes a key-value pair to the distributed key-value store.
     * 
     * @param string $key Key to write.
     * @param string $value Value to write.
     * @param int|null $version Optional version constraint.
     * @return bool True if successful, throws Exception on failure.
     */
    public function put(string $key, string $value, ?int $version = null): bool {
        $cmd = escapeshellcmd($this->clientBin) . ' --ctrler-addr ' . escapeshellarg($this->ctrlerAddr) . ' put ' . escapeshellarg($key) . ' ' . escapeshellarg($value);
        if ($version !== null) {
            $cmd .= ' ' . escapeshellarg((string)$version);
        }

        $output = [];
        $returnCode = 0;
        exec($cmd . ' 2>&1', $output, $returnCode);

        if ($returnCode !== 0) {
            throw new Exception("DKV Put failed: " . implode("\n", $output));
        }

        return trim(implode("\n", $output)) === 'OK';
    }

    /**
     * Reads a key's value and version from the distributed store.
     * 
     * @param string $key Key to read.
     * @return array Array containing 'value' and 'version'. Returns 'value' => null and 'version' => 0 if not found.
     */
    public function get(string $key): array {
        $cmd = escapeshellcmd($this->clientBin) . ' --ctrler-addr ' . escapeshellarg($this->ctrlerAddr) . ' get ' . escapeshellarg($key);

        $output = [];
        $returnCode = 0;
        exec($cmd . ' 2>&1', $output, $returnCode);

        $resultStr = trim(implode("\n", $output));

        if ($returnCode !== 0) {
            if (strpos($resultStr, 'Key not found') !== false || $returnCode === 1) {
                return ['value' => null, 'version' => 0];
            }
            throw new Exception("DKV Get failed: " . $resultStr);
        }

        if (strpos($resultStr, 'Value: ') === 0) {
            $valPart = substr($resultStr, strlen('Value: '));
            $parts = explode(' (Version: ', $valPart);
            $val = $parts[0];
            $ver = (int)substr($parts[1], 0, -1);
            return ['value' => $val, 'version' => $ver];
        }

        return ['value' => null, 'version' => 0];
    }
}
