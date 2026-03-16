import type { BitField, OrganizationVaultCreateMode } from '@proton/pass/types';

export type OrganizationSettings = {
    ShareMode: BitField;
    ItemShareMode: BitField;
    SecureLinkMode: BitField;
    ForceLockSeconds: number;
    ExportMode: BitField;
    VaultCreateMode: OrganizationVaultCreateMode;
    PasswordPolicy: Record<string, any> | null;
    AliasCreation: BitField;
    PauseListEntries: any[];
};
