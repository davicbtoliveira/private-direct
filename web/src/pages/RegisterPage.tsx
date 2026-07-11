import { useRef, useState, type FormEvent } from "react";
import { useNavigate } from "react-router-dom";
import { api } from "../api/client";
import { useSession } from "../session/sessionContext";
import AuthShell from "./AuthShell";
import styles from "./AuthShell.module.css";

const usernamePattern = /^[a-z0-9][a-z0-9._-]{2,31}$/;
const usernameGuidance =
  "Use 3-32 letters, numbers, dots, dashes, or underscores; start with a letter or number.";
const passwordGuidance = "Use 12-72 UTF-8 bytes.";

function validPassword(password: string) {
  const bytes = new TextEncoder().encode(password).length;
  return bytes >= 12 && bytes <= 72;
}

type FormErrors = {
  invite?: string;
  username?: string;
  password?: string;
  confirmation?: string;
  form?: string;
};

function errorCode(error: unknown) {
  if (typeof error === "object" && error !== null && "code" in error) {
    return String(error.code);
  }
  return "request_failed";
}

export default function RegisterPage() {
  const { login } = useSession();
  const navigate = useNavigate();
  const [inviteCode, setInviteCode] = useState("");
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [errors, setErrors] = useState<FormErrors>({});
  const [pending, setPending] = useState(false);
  const inviteRef = useRef<HTMLInputElement>(null);
  const usernameRef = useRef<HTMLInputElement>(null);
  const passwordRef = useRef<HTMLInputElement>(null);
  const confirmationRef = useRef<HTMLInputElement>(null);
  const usernameTouched = useRef(false);
  const passwordTouched = useRef(false);
  const confirmationTouched = useRef(false);

  const showErrors = (nextErrors: FormErrors) => {
    setErrors(nextErrors);
    if (nextErrors.invite) inviteRef.current?.focus();
    else if (nextErrors.username) usernameRef.current?.focus();
    else if (nextErrors.password) passwordRef.current?.focus();
    else if (nextErrors.confirmation) confirmationRef.current?.focus();
  };

  const updateFieldError = (
    field: "invite" | "username" | "password" | "confirmation",
    message?: string
  ) => {
    setErrors((current) => ({ ...current, [field]: message, form: undefined }));
  };

  const onSubmit = async (event: FormEvent) => {
    event.preventDefault();
    const canonicalInvite = inviteCode.trim();
    const canonicalUsername = username.trim().toLowerCase();
    const nextErrors: FormErrors = {};

    if (!canonicalInvite) nextErrors.invite = "Enter an invite code.";
    if (!usernamePattern.test(canonicalUsername)) {
      nextErrors.username = usernameGuidance;
    }
    if (!validPassword(password)) {
      nextErrors.password = passwordGuidance;
    }
    if (confirmation !== password) nextErrors.confirmation = "Passwords do not match.";
    if (!confirmation) nextErrors.confirmation = "Confirm your password.";

    if (Object.keys(nextErrors).length > 0) {
      showErrors(nextErrors);
      return;
    }

    setErrors({});
    setPending(true);
    try {
      await api.register(canonicalInvite, canonicalUsername, password);
    } catch (error) {
      const code = errorCode(error);
      let apiErrors: FormErrors;
      if (code === "invalid_invite") {
        apiErrors = { invite: "Invite code is not valid." };
      } else if (code === "invite_used") {
        apiErrors = {
          invite: "Invite code has already been used. Ask the operator for another.",
        };
      } else if (code === "username_exists") {
        apiErrors = { username: "Username is already registered." };
      } else if (code === "invalid_username" || code === "username_required") {
        apiErrors = { username: usernameGuidance };
      } else if (code === "invalid_password" || code === "password_required") {
        apiErrors = { password: passwordGuidance };
      } else if (code === "invite_code_required") {
        apiErrors = { invite: "Enter an invite code." };
      } else {
        apiErrors = { form: "Registration failed. Try again." };
      }
      showErrors(apiErrors);
      setPending(false);
      return;
    }

    try {
      await login(canonicalUsername, password);
      setPassword("");
      setConfirmation("");
      navigate("/chat", { replace: true });
    } catch {
      setPassword("");
      setConfirmation("");
      navigate("/login", {
        replace: true,
        state: { registrationCreated: true, username: canonicalUsername },
      });
    } finally {
      setPending(false);
    }
  };

  return (
    <AuthShell
      title="Register"
      footerLabel="Already registered?"
      footerTo="/login"
      footerLink="Sign in"
    >
      <form
        className={styles.form}
        aria-label="Registration form"
        onSubmit={onSubmit}
        noValidate
      >
        <label className={styles.field}>
          <span className={styles.label}>Invite code</span>
          <input
            ref={inviteRef}
            className={styles.input}
            name="invite_code"
            autoComplete="one-time-code"
            aria-label="Invite code"
            aria-invalid={Boolean(errors.invite)}
            aria-describedby={errors.invite ? "invite-error" : undefined}
            value={inviteCode}
            onChange={(event) => {
              setInviteCode(event.target.value);
              updateFieldError("invite");
            }}
            disabled={pending}
            required
          />
          {errors.invite && (
            <span id="invite-error" className={styles.fieldError} role="alert">
              {errors.invite}
            </span>
          )}
        </label>
        <label className={styles.field}>
          <span className={styles.label}>Username</span>
          <input
            ref={usernameRef}
            className={styles.input}
            name="username"
            autoComplete="username"
            autoCapitalize="none"
            spellCheck={false}
            aria-label="Username"
            aria-invalid={Boolean(errors.username)}
            aria-describedby={errors.username ? "username-error" : "username-hint"}
            value={username}
            onChange={(event) => {
              const nextUsername = event.target.value;
              setUsername(nextUsername);
              if (usernameTouched.current) {
                const canonical = nextUsername.trim().toLowerCase();
                updateFieldError(
                  "username",
                  usernamePattern.test(canonical) ? undefined : usernameGuidance
                );
              }
            }}
            onBlur={() => {
              usernameTouched.current = true;
              const canonical = username.trim().toLowerCase();
              setUsername(canonical);
              updateFieldError(
                "username",
                usernamePattern.test(canonical) ? undefined : usernameGuidance
              );
            }}
            disabled={pending}
            required
          />
          {errors.username ? (
            <span id="username-error" className={styles.fieldError} role="alert">
              {errors.username}
            </span>
          ) : (
            <span id="username-hint" className={styles.hint}>
              Enter your handle without @.
            </span>
          )}
        </label>
        <label className={styles.field}>
          <span className={styles.label}>Password</span>
          <input
            ref={passwordRef}
            className={styles.input}
            type="password"
            name="password"
            autoComplete="new-password"
            aria-label="Password"
            aria-invalid={Boolean(errors.password)}
            aria-describedby={errors.password ? "password-error" : "password-hint"}
            value={password}
            onChange={(event) => {
              const nextPassword = event.target.value;
              setPassword(nextPassword);
              if (passwordTouched.current) {
                updateFieldError(
                  "password",
                  validPassword(nextPassword) ? undefined : passwordGuidance
                );
              }
              if (confirmationTouched.current) {
                updateFieldError(
                  "confirmation",
                  confirmation === nextPassword ? undefined : "Passwords do not match."
                );
              }
            }}
            onBlur={() => {
              passwordTouched.current = true;
              updateFieldError(
                "password",
                validPassword(password) ? undefined : passwordGuidance
              );
            }}
            disabled={pending}
            required
          />
          {errors.password ? (
            <span id="password-error" className={styles.fieldError} role="alert">
              {errors.password}
            </span>
          ) : (
            <span id="password-hint" className={styles.hint}>
              12-72 UTF-8 bytes.
            </span>
          )}
        </label>
        <label className={styles.field}>
          <span className={styles.label}>Confirm password</span>
          <input
            ref={confirmationRef}
            className={styles.input}
            type="password"
            name="confirm_password"
            autoComplete="new-password"
            aria-label="Confirm password"
            aria-invalid={Boolean(errors.confirmation)}
            aria-describedby={errors.confirmation ? "confirmation-error" : undefined}
            value={confirmation}
            onChange={(event) => {
              const nextConfirmation = event.target.value;
              setConfirmation(nextConfirmation);
              if (confirmationTouched.current) {
                updateFieldError(
                  "confirmation",
                  nextConfirmation === password ? undefined : "Passwords do not match."
                );
              }
            }}
            onBlur={() => {
              confirmationTouched.current = true;
              updateFieldError(
                "confirmation",
                confirmation === password ? undefined : "Passwords do not match."
              );
            }}
            disabled={pending}
            required
          />
          {errors.confirmation && (
            <span id="confirmation-error" className={styles.fieldError} role="alert">
              {errors.confirmation}
            </span>
          )}
        </label>
        {errors.form && (
          <p className={styles.error} role="alert">
            {errors.form}
          </p>
        )}
        <button type="submit" className={styles.submit} disabled={pending}>
          {pending ? "Creating account…" : "Register"}
        </button>
      </form>
    </AuthShell>
  );
}
