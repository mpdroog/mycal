// Global type declarations for MyCal

// Fuse.js types
declare const Fuse: new <T>(items: T[], options: FuseOptions) => FuseInstance<T>;

interface FuseOptions {
    keys: string[];
    threshold: number;
    distance: number;
    includeScore: boolean;
    minMatchCharLength: number;
    ignoreLocation: boolean;
}

interface FuseResult<T> {
    item: T;
    score?: number;
}

interface FuseInstance<T> {
    search(query: string): FuseResult<T>[];
}

// Global functions from base.html
declare function showError(message: string, details: string): void;
