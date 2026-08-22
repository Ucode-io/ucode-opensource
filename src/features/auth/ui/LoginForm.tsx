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
  useLoginWithOtp,
  useSendCode,
} from "../api/auth";
import type {
  Connection,
  Credentials,
  LoginContext,
  LoginResult,
  OtpCredentials,
} from "../model/types";
import { AuthCard } from "./AuthCard";
import { ErrorText } from "./ErrorText";
import { GoogleButton } from "./GoogleButton";
import { ConnectionPicker } from "./ConnectionPicker";

/**
 * Вход. Один шаг, если логин ведёт ровно в одно рабочее пространство;
 * два — если бэкенд вернул connection'ы и нужно выбрать запись в каждой.
 *
 * Способы: пароль, код в SMS, код на почту, Google — те же четыре, что
 * и в старой админке, и те же, что понимает ручка входа (session_v2.go,
 * V3MultiCompanyLogin: default, WithPhone, WithEmail, WithGoogle).
 * У кода свой второй шаг — сначала получатель и «получить код», потом
 * сам код; дальше все способы сходятся в один default-login.
 */
/** Способы входа. Порядок — как в переключателе. */
const MODES = ["password", "phone", "email"] as const;

type Mode = (typeof MODES)[number];

export function LoginForm({ onSuccess }: { onSuccess: () => void }) {
  const { t } = useTranslation();
  const [mode, setMode] = useState<Mode>("password");
  const [credentials, setCredentials] = useState<Credentials>({ username: "", password: "" });
  /** Телефон или почта — смотря каким кодом входят. */
  const [recipient, setRecipient] = useState("");
  const [otp, setOtp] = useState("");
  /** Идентификатор отправленного кода. Пусто — код ещё не запрашивали. */
  const [smsId, setSmsId] = useState("");
  const [invalid, setInvalid] = useState(false);
  const [pendingChoice, setPendingChoice] = useState<{
    connections: Connection[];
    context: LoginContext;
  } | null>(null);

  const login = useLogin();
  const google = useLoginWithGoogle();
  const sendCode = useSendCode();
  const otpLogin = useLoginWithOtp();
  const withConnections = useLoginWithConnections();

  const busy =
    login.isPending ||
    google.isPending ||
    sendCode.isPending ||
    otpLogin.isPending ||
    withConnections.isPending;
  const error =
    login.error ?? google.error ?? sendCode.error ?? otpLogin.error ?? withConnections.error;

  // Человек набирает с пробелами и скобками, ручка ждёт голые +цифры;
  // у почты по краям остаются пробелы после вставки из буфера.
  const normalized = mode === "phone" ? recipient.replace(/[\s()-]/g, "") : recipient.trim();
  const otpAuth: OtpCredentials =
    mode === "phone"
      ? { type: "phone", phone: normalized, otp: otp.trim(), sms_id: smsId }
      : { type: "email", email: normalized, otp: otp.trim(), sms_id: smsId };

  const handle = (result: LoginResult) =>
    result.kind === "session"
      ? onSuccess()
      : setPendingChoice({ connections: result.connections, context: result.context });

  const switchMode = (next: Mode) => {
    setMode(next);
    /* Получатель и код — от прежнего способа: телефон в поле почты
       читается как поломка формы. */
    setRecipient("");
    setSmsId("");
    setOtp("");
    // Ошибка прежнего способа не должна висеть над чужой формой.
    login.reset();
    google.reset();
    sendCode.reset();
    otpLogin.reset();
    setInvalid(false);
  };

  const changeRecipient = () => {
    setSmsId("");
    setOtp("");
    sendCode.reset();
    otpLogin.reset();
  };

  const request = () =>
    sendCode.mutate(
      { recipient: normalized, channel: mode === "phone" ? "PHONE" : "EMAIL" },
      { onSuccess: setSmsId },
    );

  const submit = (event: FormEvent) => {
    event.preventDefault();

    if (mode === "password") {
      login.mutate(credentials, { onSuccess: handle });
      return;
    }

    /*
     * Формат проверяет и бэкенд (util.IsValidPhone, util.IsValidEmail),
     * но его ответ — 400 после запроса. Плюс и двенадцать цифр, как
     * и собаку с точкой, можно сказать сразу.
     */
    const valid = mode === "phone" ? /^\+\d{12}$/.test(normalized) : /^[^@\s]+@[^@\s]+\.[^@\s]+$/.test(normalized);
    if (!valid) {
      setInvalid(true);
      return;
    }
    setInvalid(false);

    if (!smsId) request();
    else otpLogin.mutate(otpAuth, { onSuccess: handle });
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
              credentials: mode === "password" ? credentials : otpAuth,
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
        {/* Пароль, код в SMS, код на почту — способы, а не поля одной
            формы. */}
        <div className="grid grid-cols-3 gap-1 rounded-lg bg-surface-hover p-1 text-sm">
          {MODES.map((item) => (
            <button
              key={item}
              type="button"
              onClick={() => switchMode(item)}
              className={`h-8 rounded-md transition-colors ${
                mode === item ? "bg-surface font-medium text-fg" : "text-fg-muted hover:text-fg"
              }`}
            >
              {t(`auth.${item}` as const)}
            </button>
          ))}
        </div>

        {mode === "password" ? (
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
            {/* После отправки кода получатель запирается: код привязан
                к нему, и смена номера — это новый код, а не правка поля. */}
            <Field
              label={t(mode === "phone" ? "auth.phone" : "auth.email")}
              {...(smsId
                ? {
                    action: (
                      <button
                        type="button"
                        onClick={changeRecipient}
                        className="text-xs text-fg-muted hover:text-fg"
                      >
                        {t("auth.changeRecipient")}
                      </button>
                    ),
                  }
                : {})}
            >
              <Input
                type={mode === "phone" ? "tel" : "email"}
                autoComplete={mode === "phone" ? "tel" : "email"}
                required
                placeholder={
                  mode === "phone" ? "+998 90 123 45 67" : t("auth.emailPlaceholder")
                }
                value={recipient}
                disabled={Boolean(smsId)}
                onChange={(e) => setRecipient(e.target.value)}
              />
            </Field>

            {invalid && (
              <p className="text-xs text-danger">
                {t(mode === "phone" ? "auth.phoneInvalid" : "auth.emailInvalid")}
              </p>
            )}

            {smsId && (
              <Field label={t(mode === "phone" ? "auth.otpCode" : "auth.recoverCode")}>
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
          {mode !== "password" && !smsId
            ? sendCode.isPending
              ? t("auth.sendingCode")
              : t("auth.sendCode")
            : busy
              ? t("auth.signingIn")
              : t("auth.signIn")}
        </Button>

        {mode !== "password" && smsId && (
          <button
            type="button"
            disabled={busy}
            onClick={request}
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
