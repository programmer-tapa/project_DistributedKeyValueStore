import sys
import os

# Dynamic import helper: Add the repository root directory to sys.path
# to allow clean importing from the 'library' folder.
repo_root = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
sys.path.append(repo_root)

from library.dkv import DKVClient

def load_env():
    env_path = os.path.join(repo_root, ".env")
    if os.path.exists(env_path):
        with open(env_path, "r") as f:
            for line in f:
                line = line.strip()
                if not line or line.startswith("#"):
                    continue
                if "=" in line:
                    key, val = line.split("=", 1)
                    key = key.strip()
                    if key not in os.environ:
                        os.environ[key] = val.strip()

def main():
    load_env()
    # Read controller address from environment variable (default to kvsrv:9000 on the docker network)
    ctrler_addr = os.environ.get("DKV_CTRLED_ADDR", "kvsrv:9000")
    
    # Path to the compiled Go CLI client binary relative to the repository root
    client_bin = os.path.join(repo_root, "DistributedKeyValueStore", "bin", "dkv-client")

    print(f"Initializing DKV Client...")
    print(f"  Controller Address: {ctrler_addr}")
    print(f"  Client Binary: {client_bin}\n")

    client = DKVClient(ctrler_addr=ctrler_addr, client_bin=client_bin)

    # 1. Put value
    key = "user_101_session"
    value = "active_authenticated"
    print(f"Executing PUT: '{key}' => '{value}'...")
    try:
        success = client.put(key, value)
        print(f"  -> Success: {success}\n")
    except Exception as e:
        print(f"  -> Error executing PUT: {e}\n")
        return

    # 2. Get value
    print(f"Executing GET: '{key}'...")
    try:
        val, version = client.get(key)
        print(f"  -> Value: '{val}'")
        print(f"  -> Version: {version}\n")
    except Exception as e:
        print(f"  -> Error executing GET: {e}\n")
        return

    # 3. Get non-existent key
    missing_key = "non_existent_key"
    print(f"Executing GET for non-existent key: '{missing_key}'...")
    try:
        val, version = client.get(missing_key)
        print(f"  -> Value: {val} (Should be None)")
        print(f"  -> Version: {version} (Should be 0)\n")
    except Exception as e:
        print(f"  -> Error executing GET: {e}\n")
        return

if __name__ == "__main__":
    main()
