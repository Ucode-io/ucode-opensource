import { useState, type FormEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";
import { Divider } from "@/shared/ui/divider";
import { Field, Input } from "@/shared/ui/input";
import { PasswordInput } from "@/shared/ui/password-input";
import {
  useLogin,
  useLoginWithConnections,
  useLoginWithGoogle,
  useLoginWithPhone,
  useSendPhoneCode,
} from "../api/auth";
import type {
  Connection,
  Credentials,
  LoginContext,
  LoginResult,
  PhoneCredentials,
} from "../model/types";
import { AuthCard } from "./AuthCard";
import { ErrorText } from "./ErrorText";
import { GoogleButton } from "./GoogleButton";
import { ConnectionPicker } from "./ConnectionPicker";

/**
 * Вход. Один шаг, если логин ведёт ровно в одно рабочее пространство;
 * два — если бэкенд вернул connection'ы и нужно выбрать запись в каждой.
 *
 * Способы: почта с паролем, телефон с кодом из SMS, Google. У телефона
 * свой второй шаг — сначала номер и «получить код», потом сам код;
 * дальше все способы сходятся в один default-login.
 */
export function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<"email" | "phone">("email");
  const [credentials, setCredentials] = useState<Credentials>({ username: "", password: "" });
  const [phone, setPhone] = useState("");
  const [otp, setOtp] = useState("");
  /** Идентификатор отправленного кода. Пусто — код ещё не запрашивали. */
  const [smsId, setSmsId] = useState("");
  const [phoneInvalid, setPhoneInvalid] = useState(false);
  const [pendingChoice, setPendingChoice] = useState<{
    connections: Connection[];
    context: LoginContext;
  } | null>(null);

  const login = useLogin();
  const google = useLoginWithGoogle();
  const sendCode = useSendPhoneCode();
  const phoneLogin = useLoginWithPhone();
  const withConnections = useLoginWithConnections();

  const busy =
    login.isPending ||
    google.isPending ||
    sendCode.isPending ||
    phoneLogin.isPending ||
    withConnections.isPending;
  const error =
    login.error ?? google.error ?? sendCode.error ?? phoneLogin.error ?? withConnections.error;

  // Человек набирает с пробелами и скобками, ручка ждёт голые +цифры.
  const normalized = phone.replace(/[\s()-]/g, "");
  const phoneAuth: PhoneCredentials = {
    type: "phone",
    phone: normalized,
    otp: otp.trim(),
    sms_id: smsId,
  };

  const handle = (result: LoginResult) =>
    result.kind === "session"
      ? onSuccess()
      : setPendingChoice({ connections: result.connections, context: result.context });

  const switchMode = (next: "email" | "phone") => {
    setMode(next);
    // Ошибка прежнего способа не должна висеть над чужой формой.
    login.reset();
    google.reset();
    sendCode.reset();
    phoneLogin.reset();
    setPhoneInvalid(false);
  };

  const changePhone = () => {
    setSmsId("");
    setOtp("");
    sendCode.reset();
    phoneLogin.reset();
  };

  const submit = (event: FormEvent) => {
    event.preventDefault();

    if (mode === "email") {
      login.mutate(credentials, { onSuccess: handle });
      return;
    }

    /*
     * Формат проверяет и бэкенд (util.IsValidPhone), но его ответ —
     * 400 после запроса. Плюс и двенадцать цифр можно сказать сразу.
     */
    if (!/^\+\d{12}$/.test(normalized)) {
      setPhoneInvalid(true);
      return;
    }
    setPhoneInvalid(false);

    if (!smsId) sendCode.mutate(normalized, { onSuccess: setSmsId });
    else phoneLogin.mutate(phoneAuth, { onSuccess: handle });
  };

  if (pendingChoice) {
    return (
      <ConnectionPicker
        connections={pendingChoice.connections}
        busy={busy}
        error={error}
        onSubmit={(selection) =>
          withConnections.mutate(
            {
              // Чем входили — тем и подтверждаем выбор: /v2/login
              // проверяет пароль или код заново.
              credentials: mode === "phone" ? phoneAuth : credentials,
              selection,
              ...pendingChoice,
            },
            { onSuccess: handle },
          )
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
        {/* Почта или телефон — способы, а не поля одной формы. */}
        <div className="grid grid-cols-2 gap-1 rounded-lg bg-surface-hover p-1 text-sm">
          {(["email", "phone"] as const).map((item) => (
            <button
              key={item}
              type="button"
              onClick={() => switchMode(item)}
              className={`h-8 rounded-md transition-colors ${
                mode === item ? "bg-surface font-medium text-fg" : "text-fg-muted hover:text-fg"
              }`}
            >
              {t(item === "email" ? "auth.email" : "auth.phone")}
            </button>
          ))}
        </div>

        {mode === "email" ? (
          <>
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
          </>
        ) : (
          <>
            {/* После отправки кода номер запирается: код привязан к нему,
                и смена номера — это новый код, а не правка поля. */}
            <Field
              label={t("auth.phone")}
              {...(smsId
                ? {
                    action: (
                      <button
                        type="button"
                        onClick={changePhone}
                        className="text-xs text-fg-muted hover:text-fg"
                      >
                        {t("auth.changePhone")}
                      </button>
                    ),
                  }
                : {})}
            >
              <Input
                type="tel"
                autoComplete="tel"
                required
                placeholder="+998 90 123 45 67"
                value={phone}
                disabled={Boolean(smsId)}
                onChange={(e) => setPhone(e.target.value)}
              />
            </Field>

            {phoneInvalid && <p className="text-xs text-danger">{t("auth.phoneInvalid")}</p>}

            {smsId && (
              <Field label={t("auth.otpCode")}>
                <Input
                  inputMode="numeric"
                  autoComplete="one-time-code"
                  required
                  autoFocus
                  placeholder="0000"
                  value={otp}
                  onChange={(e) => setOtp(e.target.value)}
                />
              </Field>
            )}
          </>
        )}

        {error && <ErrorText error={error} />}

        <Button type="submit" disabled={busy} className="w-full">
          {mode === "phone" && !smsId
            ? sendCode.isPending
              ? t("auth.sendingCode")
              : t("auth.sendCode")
            : busy
              ? t("auth.signingIn")
              : t("auth.signIn")}
        </Button>

        {mode === "phone" && smsId && (
          <button
            type="button"
            disabled={busy}
            onClick={() => sendCode.mutate(normalized, { onSuccess: setSmsId })}
            className="self-start text-xs text-fg-muted transition-colors hover:text-fg"
          >
            {t("auth.resendCode")}
          </button>
        )}

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
