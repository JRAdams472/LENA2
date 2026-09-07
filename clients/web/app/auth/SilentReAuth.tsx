"use client";

import { useGoogleOneTapLogin } from "@react-oauth/google";
import { useAuth } from "@/app/auth/AuthProvider";

/**
 * Attempts silent re-authentication via Google Identity Services One Tap.
 *
 * With `auto_select`, GIS returns a fresh ID token without any UI when the
 * user has a single eligible Google session that previously signed in —
 * this covers the ~1 h ID-token lifetime so an expired sessionStorage token
 * (or a 401 that signed the user out) is renewed transparently. When no
 * session is eligible the One Tap prompt may appear; it is dismissible and
 * the user simply sees the normal login screen.
 *
 * Must be rendered inside both GoogleOAuthProvider (GIS script/context) and
 * AuthProvider (useAuth).
 */
export default function SilentReAuth() {
  const { isAuthenticated, signIn } = useAuth();

  useGoogleOneTapLogin({
    disabled: isAuthenticated,
    auto_select: true,
    cancel_on_tap_outside: true,
    onSuccess: (response) => {
      if (response.credential) {
        signIn(response.credential);
      }
    },
  });

  return null;
}
