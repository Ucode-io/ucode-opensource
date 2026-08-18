import { useState, type FormEvent } from "react";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Field, Input } from "@/shared/ui/input";
import { PasswordInput } from "@/shared/ui/password-input";
import { useAcceptInvite } from "../api/auth";
import type { Invite } from "../model/types";
import { AuthCard } from "./AuthCard";
import { ErrorText } from "./ErrorText";

/**
 * Регистрация по приглашению.
 *
 * Всё, кроме логина и пароля, уже известно из ссылки: проект,
 * окружение, роль и тип клиента. Поэтому здесь два поля и одна кнопка —
 * человек, пришедший по приглашению, не выбирает ни компанию, ни роль.
 *
 * Почта не спрашивается: приглашение уже пришло на неё, а бэкенд при
 * `type: "login"` её и не требует.
 */
export function InviteForm({ invite, onSuccess }: { invite: Invite; onSuccess: () => void }) {
  const { t } = useTranslation();
  const [credentials, setCredentials] = useState({ username: "", password: "" });

  const accept = useAcceptInvite();

  const submit = (event: FormEvent) => {
    event.preventDefault();
    accept.mutate({ credentials, invite }, { onSuccess });
  };

  return (
    <AuthCard
      title={t("auth.inviteTitle")}
      subtitle={t("auth.inviteSubtitle")}
      footer={t("auth.inviteFooter")}
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Field label={t("auth.login")} hint={t("auth.loginHint")}>
          <Input
            autoFocus
            required
            minLength={6}
            autoComplete="username"
            value={credentials.username}
            onChange={(event) =>
              setCredentials((value) => ({ ...value, username: event.target.value }))
            }
          />
        </Field>

        <Field label={t("auth.password")}>
          <PasswordInput
            required
            autoComplete="new-password"
            placeholder="••••••••"
            value={credentials.password}
            onChange={(event) =>
              setCredentials((value) => ({ ...value, password: event.target.value }))
            }
          />
        </Field>

        {accept.error && <ErrorText error={accept.error} />}

        <Button type="submit" disabled={accept.isPending} className="w-full">
          {accept.isPending ? t("auth.signingUp") : t("auth.inviteSubmit")}
        </Button>
      </form>
    </AuthCard>
  );
}
