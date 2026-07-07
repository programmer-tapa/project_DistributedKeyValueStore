import subprocess
import os

class DKVClient:
    def __init__(self, ctrler_addr="localhost:9000", client_bin="dkv-client"):
        """
        Initializes the DKV client wrapper.
        
        :param ctrler_addr: Address of the Shard Controller or config store.
        :param client_bin: Path to the compiled dkv-client CLI binary.
        """
        self.ctrler_addr = ctrler_addr
        self.client_bin = client_bin

    def put(self, key: str, value: str, version: int = None) -> bool:
        """
        Writes a key-value pair to the distributed key-value store.
        
        :param key: Key to write.
        :param value: Value to write.
        :param version: Optional version constraint.
        :return: True if successful, raises Exception otherwise.
        """
        cmd = [self.client_bin, "--ctrler-addr", self.ctrler_addr, "put", key, value]
        if version is not None:
            cmd.append(str(version))
        
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            raise Exception(f"DKV Put failed: {result.stderr.strip()}")
        return result.stdout.strip() == "OK"

    def get(self, key: str) -> tuple[str, int]:
        """
        Reads a key's value and version from the distributed store.
        
        :param key: Key to read.
        :return: A tuple of (value, version). If key is not found, returns (None, 0).
        """
        cmd = [self.client_bin, "--ctrler-addr", self.ctrler_addr, "get", key]
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            stderr = result.stderr.strip()
            if "Key not found" in stderr or result.returncode == 1:
                return None, 0
            raise Exception(f"DKV Get failed: {stderr}")
        
        output = result.stdout.strip()
        if output.startswith("Value:"):
            try:
                # Expected format: "Value: <val> (Version: <ver>)"
                val_part, ver_part = output.split(" (Version: ")
                val = val_part[len("Value: "):]
                ver = int(ver_part[:-1])
                return val, ver
            except Exception:
                raise Exception(f"Failed to parse DKV client output: {output}")
        return None, 0
