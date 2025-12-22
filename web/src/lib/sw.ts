export const SW_MESSAGE_TYPE = {
    SKIP_WAITING: 'SKIP_WAITING',
    CLEAR_CACHE: 'CLEAR_CACHE',
    CACHE_CLEARED: 'CACHE_CLEARED',
} as const;

export type SwMessageType = (typeof SW_MESSAGE_TYPE)[keyof typeof SW_MESSAGE_TYPE];

// Keep in sync with `web/public/sw.js`
export const OCTOPUS_CACHE_PREFIX = 'octopus-';

export function isOctopusCacheName(name: string) {
    return name.startsWith(OCTOPUS_CACHE_PREFIX);
}


