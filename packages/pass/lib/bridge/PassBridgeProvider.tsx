import type { ReactNode } from 'react';
import { createContext, useContext } from 'react';

const PassBridgeContext = createContext<any>({});

export const PassBridgeProvider = ({ children }: { children: ReactNode }) => children;
export const usePassBridge = () => useContext(PassBridgeContext);
