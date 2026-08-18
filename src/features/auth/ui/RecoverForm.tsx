import { useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Field, Input } from "@/shared/ui/input";
import { PasswordInput } from "@/shared/ui/password-input";
import {
  useSendCodeToEmail,
  useSetPassword,
  useStartRecovery,
  useVerifyCode,
} from "../api/auth";
import type { RecoveryStart } from "../model/types";
import { AuthCard } from "./AuthCard";
import { ErrorText } from "./ErrorText";

/**
 * Восстановление пароля. Четыре шага бэкенда — четыре экрана одной
 * формы, а не четыре страницы: назад тут возвращаться некуда, а код
 * из письма живёт пять часов и переживает случайный F5 только вместе
 * с адресом, которого у нас нет.
 *
 * Шаг «почта» появляется не всегда: у пользователя, заведённого без
 * email, отправлять код некуда, и его сначала спрашивают. Так же ведёт
 * себя старая админка (Auth/components/RecoverPassword.jsx).
 */
type Step =
  | { name: "login" }
  | { name: "email"; userId: string }
  | { name: "code"; userId: string; smsId: string; email: string }
  | { name: "password"; userId: string };

export function RecoverForm({ onDone }: { onDone: () => void }) {
  const { t } = useTranslation();

  const [step, setStep] = useState<Step>({ name: "login" });
  const [login, setLogin] = useState("");
  const [email, setEmail] = useState("");
  const [code, setCode] = useState("");
  const [password, setPassword] = useState("");
  /** Неверный код — не ошибка запроса: сервер отвечает 200 и `false`. */
  const [wrongCode, setWrongCode] = useState(false);
  /** Логина нет — тоже 200, просто без user_id. */
  const [unknownLogin, setUnknownLogin] = useState(false);

  const start = useStartRecovery();
  const sendCode = useSendCodeToEmail();
  const verify = useVerifyCode();
  const save = useSetPassword();

  const busy = start.isPending || sendCode.isPending || verify.isPending || save.isPending;
  const error = start.error ?? sendCode.error ?? verify.error ?? save.error;

  /** Первый шаг и повтор отправки кода приводят к одному и тому же. */
  const applyStart = (result: RecoveryStart) => {
    if (result.kind === "unknown") {
      setUnknownLogin(true);
      return;
    }

    setStep(
      result.kind === "sent"
        ? { name: "code", userId: result.userId, smsId: result.smsId, email: result.email }
        : { name: "email", userId: result.userId },
    );
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();
    setUnknownLogin(false);
    setWrongCode(false);

    switch (step.name) {
      case "login":
        start.mutate(login, { onSuccess: applyStart });
        return;
      case "email":
        sendCode.mutate({ userId: step.userId, email }, { onSuccess: applyStart });
        return;
      case "code":
        verify.mutate(
          { smsId: step.smsId, otp: code },
          {
            onSuccess: (verified) =>
              verified ? setStep({ name: "password", userId: step.userId }) : setWrongCode(true),
          },
        );
        return;
      case "password":
        save.mutate({ userId: step.userId, password }, { onSuccess: onDone });
    }
  };

  return (
    <AuthCard
      title={t("auth.recoverTitle")}
      subtitle={
        step.name === "login"
          ? t("auth.recoverLoginSubtitle")
          : step.name === "email"
            ? t("auth.recoverEmailSubtitle")
            : step.name === "code"
              ? t("auth.recoverCodeSubtitle", { email: step.email })
              : t("auth.recoverPasswordSubtitle")
      }
      footer={
        <Link to="/login" className="font-medium text-accent-text hover:underline">
          {t("auth.backToSignIn")}
        </Link>
      }
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        {step.name === "login" && (
          <Field label={t("auth.login")}>
            <Input
              autoFocus
              required
              autoComplete="username"
              value={login}
              onChange={(event) => setLogin(event.target.value)}
            />
          </Field>
        )}

        {step.name === "email" && (
          <Field label={t("auth.email")} hint={t("auth.recoverEmailHint")}>
            <Input
              autoFocus
              required
              type="email"
              autoComplete="email"
              placeholder={t("auth.emailPlaceholder")}
              value={email}
              onChange={(event) => setEmail(event.target.value)}
            />
          </Field>
        )}

        {step.name === "code" && (
          <Field label={t("auth.recoverCode")}>
            <Input
              autoFocus
              required
              /* Код цифровой, но поле текстовое: у number-поля колёсико
                 мыши меняет значение, а стрелки в углу тут ни к чему. */
              inputMode="numeric"
              autoComplete="one-time-code"
              value={code}
              onChange={(event) => setCode(event.target.value)}
            />
          </Field>
        )}

        {step.name === "password" && (
          <Field label={t("auth.recoverNewPassword")}>
            <PasswordInput
              autoFocus
              required
              autoComplete="new-password"
              placeholder="••••••••"
              value={password}
              onChange={(event) => setPassword(event.target.value)}
            />
          </Field>
        )}

        {unknownLogin && <ErrorText error={new Error(t("auth.recoverUnknownLogin"))} />}
        {wrongCode && <ErrorText error={new Error(t("auth.recoverWrongCode"))} />}
        {error && <ErrorText error={error} />}

        <Button type="submit" disabled={busy} className="w-full">
          {busy ? t("common.loading") : t(step.name === "password" ? "action.save" : "auth.continue")}
        </Button>
      </form>
    </AuthCard>
  );
}
