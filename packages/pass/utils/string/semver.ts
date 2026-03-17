export const semver = (version: string): number => {
    const parts = version.replace(/^v/, '').split('.').map(Number);
    const [major = 0, minor = 0, patch = 0] = parts;
    return major * 1_000_000 + minor * 1_000 + patch;
};
