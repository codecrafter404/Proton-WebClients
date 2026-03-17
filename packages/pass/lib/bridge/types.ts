import type { AliasMailbox, AliasOptions, MaybeNull, Share, ShareType } from '@proton/pass/types';

export type PassBridgeAliasItem = {
    aliasEmail: string;
    item: {
        createTime: number;
        data: {
            metadata: {
                name: string;
                note: string;
            };
            extraFields: any[];
            content: {
                mailboxes: AliasMailbox[];
            };
        };
        itemId: string;
    };
};

export type PassBridge = {
    vault: MaybeNull<Share<ShareType.Vault>>;
    aliases: {
        get: () => Promise<PassBridgeAliasItem[]>;
        getAliasOptions: (shareId: string) => Promise<AliasOptions>;
        getAliasCount: () => Promise<number>;
        create: (data: any) => Promise<PassBridgeAliasItem>;
    };
    user: {
        addresses: () => Promise<string[]>;
    };
    organization: {
        settings: {
            get: () => Promise<any>;
            set: (data: any) => Promise<any>;
        };
    };
};
