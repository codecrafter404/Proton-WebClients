export type Maybe<T> = T | undefined;
export type MaybeNull<T> = T | null;

export enum BitField {
    DISABLED = 0,
    ACTIVE = 1,
}

export enum OrganizationVaultCreateMode {
    ALLOWED = 0,
    ONLYORGADMINS = 1,
    ONLYORGADMINSANDPERSONALVAULT = 2,
}

export enum ShareType {
    Vault = 1,
}

export type Share<T extends ShareType = ShareType> = {
    shareId: string;
    type: T;
    [key: string]: any;
};

export type AliasMailbox = {
    ID: number;
    Email: string;
};

export type AliasOptions = {
    suffixes: { suffix: string; signedSuffix: string; isCustom: boolean; domain: string }[];
    mailboxes: AliasMailbox[];
};

export type OrganizationUpdatePasswordPolicyInput = Record<string, any>;
export type OrganizationGetResponse = Record<string, any>;
export type OrganizationUrlPauseEntryDto = Record<string, any>;
export type OrganizationUrlPauseEntryValues = Record<string, any>;
export type MemberMonitorReport = Record<string, any>;
