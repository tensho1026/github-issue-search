import type { UseQueryResult } from "@tanstack/react-query";
import { createContext, useContext } from "react";

import type {
  AuthSession,
  AuthSessionEnvelope,
} from "../../shared/api/generated";

export type AuthContextValue = {
  markSessionExpired: () => Promise<void>;
  query: UseQueryResult<AuthSessionEnvelope, Error>;
  session: AuthSession | undefined;
  signIn: (returnTo?: string) => void;
};

export const AuthContext = createContext<AuthContextValue | undefined>(
  undefined,
);

export function useAuth(): AuthContextValue {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used within AuthProvider.");
  }
  return value;
}
