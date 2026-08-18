import { useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Divider } from "@/shared/ui/divider";
import { Field, Input } from "@/shared/ui/input";
import { PasswordInput } from "@/shared/ui/password-input";
import { useLogin, useLoginWithConnections, useLoginWithGoogle } from "../api/auth";
import type { Connection, Credentials, LoginContext, LoginResult } from "../model/types";
import { AuthCard } from "./AuthCard";
import { ErrorText } from "./ErrorText";
import { GoogleButton } from "./GoogleButton";
import { ConnectionPicker } from "./ConnectionPicker";

/**
 * Вход. Один шаг, если логин ведёт ровно в одно рабочее пространство;
 * два — если бэкенд вернул connection'ы и нужно выбрать запись в каждой.
 */
export function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useTranslation();
  const [credentials, setCredentials] = useState<Credentials>({ username: "", password: "" });
  const [pendingChoice, setPendingChoice] = useState<{
    connections: Connection[];
    context: LoginContext;
  } | null>(null);

  const login = useLogin();
  const google = useLoginWithGoogle();
  const withConnections = useLoginWithConnections();

  const busy = login.isPending || google.isPending || withConnections.isPending;
  const error = login.error ?? google.error ?? withConnections.error;

  const handle = (result: LoginResult) =>
    result.kind === "session"
      ? onSuccess()
      : setPendingChoice({ connections: result.connections, context: result.context });

  const submit = (event: FormEvent) => {
    event.preventDefault();
    login.mutate(credentials, { onSuccess: handle });
  };

  if (pendingChoice) {
    return (
      <ConnectionPicker
        connections={pendingChoice.connections}
        busy={busy}
        error={error}
        onSubmit={(selection) =>
          withConnections.mutate({ credentials, selection, ...pendingChoice }, { onSuccess: handle })
        }
      />
    );
  }

  return (
    <AuthCard
      title={t("auth.signInTitle")}
      subtitle={t("auth.signInSubtitle")}
      footer={
        <>
          {t("auth.noAccount")}{" "}
          <Link to="/register" className="font-medium text-accent-text hover:underline">
            {t("auth.signUp")}
          </Link>
        </>
      }
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Field label={t("auth.email")}>
          <Input
            type="text"
            autoComplete="username"
            required
            placeholder={t("auth.emailPlaceholder")}
            value={credentials.username}
            onChange={(e) => setCredentials((c) => ({ ...c, username: e.target.value }))}
          />
        </Field>

        <Field
          label={t("auth.password")}
          /* Ссылка рядом с подписью поля, а не под кнопкой: её ищут
             в тот момент, когда пароль не вспомнился, — то есть глядя
             на это поле. */
          action={
            <Link to="/recover" className="text-xs text-fg-muted hover:text-fg">
              {t("auth.forgotPassword")}
            </Link>
          }
        >
          <PasswordInput
            autoComplete="current-password"
            required
            placeholder="••••••••"
            value={credentials.password}
            onChange={(e) => setCredentials((c) => ({ ...c, password: e.target.value }))}
          />
        </Field>

        {error && <ErrorText error={error} />}

        <Button type="submit" disabled={busy} className="w-full">
          {busy ? t("auth.signingIn") : t("auth.signIn")}
        </Button>

        <Divider>{t("auth.orContinueWith")}</Divider>

        <GoogleButton
          disabled={busy}
          label="Google"
          onToken={(token) => token && google.mutate(token, { onSuccess: handle })}
        />
      </form>
    </AuthCard>
  );
}
