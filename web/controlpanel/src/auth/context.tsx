import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from "react";
import type { ReactNode } from "react";
import { api, configureAuth } from "@/api/client";
import type { User } from "@/api/types";
import i18n from "@/i18n";

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  isLoading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  updateUser: (updates: Partial<User>) => void;
}

const AuthContext = createContext<AuthState | null>(null);

const TOKEN_KEY = "controlpanel_token";

export function AuthProvider({ children }: { children: ReactNode }) {
  const [token, setToken] = useState<string | null>(() =>
    localStorage.getItem(TOKEN_KEY),
  );
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(!!localStorage.getItem(TOKEN_KEY));
  const tokenRef = useRef(token);
  tokenRef.current = token;

  const logout = useCallback(() => {
    setToken(null);
    setUser(null);
    localStorage.removeItem(TOKEN_KEY);
  }, []);

  // Configure the API client to use our auth
  useEffect(() => {
    configureAuth(
      () => tokenRef.current,
      () => logout(),
    );
  }, [logout]);

  // Fetch user when we have a token but no user
  useEffect(() => {
    if (!token) {
      setIsLoading(false);
      return;
    }

    api
      .get<User>("/api/v1/me")
      .then(setUser)
      .catch(() => logout())
      .finally(() => setIsLoading(false));
  }, [token, logout]);

  const login = useCallback(async (email: string, password: string) => {
    const { token: newToken } = await api.post<{ token: string }>(
      "/auth/login",
      { email, password },
    );

    localStorage.setItem(TOKEN_KEY, newToken);
    setToken(newToken);
  }, []);

  const updateUser = useCallback((updates: Partial<User>) => {
    setUser((prev) => (prev ? { ...prev, ...updates } : prev));
  }, []);

  // Sync i18next language with user locale
  useEffect(() => {
    if (user?.locale && user.locale !== i18n.language) {
      i18n.changeLanguage(user.locale);
    }
  }, [user?.locale]);

  return (
    <AuthContext.Provider
      value={{
        user,
        token,
        isAuthenticated: !!token && !!user,
        isLoading,
        login,
        logout,
        updateUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}

export function useAuth() {
  const ctx = useContext(AuthContext);
  if (!ctx) throw new Error("useAuth must be used within AuthProvider");
  return ctx;
}
