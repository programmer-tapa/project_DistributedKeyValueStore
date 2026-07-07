import { execFile } from 'child_process';
import { promisify } from 'util';

const execFileAsync = promisify(execFile);

export class DKVClient {
    private ctrlerAddr: string;
    private clientBin: string;

    /**
     * Initializes the DKV client wrapper.
     * 
     * @param ctrlerAddr Address of the Shard Controller or config store.
     * @param clientBin Path to the compiled dkv-client CLI binary.
     */
    constructor(ctrlerAddr = 'localhost:9000', clientBin = 'dkv-client') {
        this.ctrlerAddr = ctrlerAddr;
        this.clientBin = clientBin;
    }

    /**
     * Writes a key-value pair to the distributed key-value store.
     * 
     * @param key Key to write.
     * @param value Value to write.
     * @param version Optional version constraint.
     * @returns Promise resolving to true if successful, rejects on failure.
     */
    async put(key: string, value: string, version?: number): Promise<boolean> {
        const args = ['--ctrler-addr', this.ctrlerAddr, 'put', key, value];
        if (version !== undefined) {
            args.push(version.toString());
        }

        try {
            const { stdout } = await execFileAsync(this.clientBin, args);
            return stdout.trim() === 'OK';
        } catch (error: any) {
            throw new Error(`DKV Put failed: ${error.stderr || error.message}`);
        }
    }

    /**
     * Reads a key's value and version from the distributed store.
     * 
     * @param key Key to read.
     * @returns Promise resolving to an object containing value and version. Returns value null and version 0 if key not found.
     */
    async get(key: string): Promise<{ value: string | null; version: number }> {
        const args = ['--ctrler-addr', this.ctrlerAddr, 'get', key];

        try {
            const { stdout } = await execFileAsync(this.clientBin, args);
            const output = stdout.trim();
            if (output.startsWith('Value:')) {
                const valPart = output.substring('Value: '.length);
                const verSplit = valPart.split(' (Version: ');
                const val = verSplit[0];
                const ver = parseInt(verSplit[1].slice(0, -1), 10);
                return { value: val, version: ver };
            }
            return { value: null, version: 0 };
        } catch (error: any) {
            if (error.code === 1 || (error.stderr && error.stderr.includes('Key not found'))) {
                return { value: null, version: 0 };
            }
            throw new Error(`DKV Get failed: ${error.stderr || error.message}`);
        }
    }
}
