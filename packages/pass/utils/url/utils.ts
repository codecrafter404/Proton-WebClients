export const intoCleanHostname = (url: string): string => {
    try {
        return new URL(url).hostname;
    } catch {
        return url;
    }
};
