import { useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Field, Input } from "@/shared/ui/input";
import { PasswordInput } from "@/shared/ui/password-input";
import { useRegister } from "../api/auth";
import type { Registration } from "../model/types";
import { PASSWORD_RULES, checkPassword, isLoginValid, isPasswordValid } from "../model/validation";
import { AuthCard } from "./AuthCard";
import { ErrorText } from "./ErrorText";

const EMPTY: Registration = { companyName: "", login: "", email: "", password: "" };

/**
 * Регистрация создаёт компанию вместе с её первым пользователем — в ucode
 * это одно действие, а не два.
 *
 * Через Google регистрации нет намеренно: бэкенд принимает Google-токен
 * без проверки подписи, и подделанный токен создал бы компанию на чужой
 * email. Вернём, когда там появится проверка.
 */
export function RegisterForm({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useTranslation();
  const [form, setForm] = useState<Registration>(EMPTY);
  const [touchedPassword, setTouchedPassword] = useState(false);

  const register = useRegister();
  const rules = checkPassword(form.password);
  const valid =
    form.companyName.trim().length > 0 &&
    isLoginValid(form.login) &&
    form.email.includes("@") &&
    isPasswordValid(form.password);

  const set = (patch: Partial<Registration>) => setForm((f) => ({ ...f, ...patch }));

  const submit = (event: FormEvent) => {
    event.preventDefault();
    register.mutate(form, { onSuccess });
  };

  return (
    <AuthCard
      title={t("auth.signUpTitle")}
      subtitle={t("auth.signUpSubtitle")}
      footer={
        <>
          {t("auth.haveAccount")}{" "}
          <Link to="/login" className="font-medium text-accent-text hover:underline">
            {t("auth.signIn")}
          </Link>
        </>
      }
    >
      <form onSubmit={submit} className="flex flex-col gap-4">
        <Field label={t("auth.companyName")} hint={t("auth.companyNameHint")}>
          <Input
            required
            autoComplete="organization"
            value={form.companyName}
            onChange={(e) => set({ companyName: e.target.value })}
          />
        </Field>

        <Field
          label={t("auth.login")}
          hint={
            form.login && !isLoginValid(form.login) ? t("auth.loginTooShort") : t("auth.loginHint")
          }
        >
          <Input
            required
            autoComplete="username"
            value={form.login}
            onChange={(e) => set({ login: e.target.value })}
          />
        </Field>

        <Field label={t("auth.email")}>
          <Input
            required
            type="email"
            autoComplete="email"
            placeholder={t("auth.emailPlaceholder")}
            value={form.email}
            onChange={(e) => set({ email: e.target.value })}
          />
        </Field>

        <Field label={t("auth.password")}>
          <PasswordInput
            required
            autoComplete="new-password"
            placeholder="••••••••"
            value={form.password}
            onBlur={() => setTouchedPassword(true)}
            onChange={(e) => set({ password: e.target.value })}
          />
        </Field>

        {(touchedPassword || form.password) && (
          <ul className="flex flex-col gap-1">
            {PASSWORD_RULES.map((rule) => (
              <li
                key={rule.key}
                className={`flex items-center gap-2 text-xs ${
                  rules[rule.key] ? "text-success" : "text-fg-subtle"
                }`}
              >
                <Check done={rules[rule.key]} />
                {t(`auth.rule.${rule.key}`)}
              </li>
            ))}
          </ul>
        )}

        {register.error && <ErrorText error={register.error} />}

        <Button type="submit" disabled={!valid || register.isPending} className="w-full">
          {register.isPending ? t("auth.signingUp") : t("auth.signUp")}
        </Button>
      </form>
    </AuthCard>
  );
}

function Check({ done }: { done: boolean }) {
  return (
    <svg width="12" height="12" viewBox="0 0 12 12" fill="none" aria-hidden>
      {done ? (
        <path d="M2.5 6.3 4.8 8.5 9.5 3.8" stroke="currentColor" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" />
      ) : (
        <circle cx="6" cy="6" r="2" fill="currentColor" opacity=".4" />
      )}
    </svg>
  );
}
