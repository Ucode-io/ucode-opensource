import { useGoogleLogin } from "@react-oauth/google";
import { useTranslation } from "react-i18next";
import { Button } from "@/shared/ui/button";

/**
 * Кнопка своя, а не гугловская: она должна выглядеть как остальной
 * интерфейс. Поэтому берём неявный OAuth-поток — он отдаёт access token,
 * который и ждёт бэкенд.
 */
export function GoogleButton({
  onToken,
  disabled,
  label,
}: {
  onToken: (accessToken: string) => void;
  disabled?: boolean;
  label: string;
}) {
  const { t } = useTranslation();

  const start = useGoogleLogin({
    onSuccess: (response) => onToken(response.access_token),
    // Ошибку показываем там же, где остальные ошибки формы.
    onError: () => onToken(""),
  });

  return (
    <Button
      type="button"
      variant="secondary"
      disabled={disabled ?? false}
      onClick={() => start()}
      aria-label={t("auth.continueWithGoogle")}
      className="w-full"
    >
      <GoogleMark />
      {label}
    </Button>
  );
}

/** Официальный четырёхцветный знак — единственное место с фиксированными цветами. */
function GoogleMark() {
  return (
    <svg width="16" height="16" viewBox="0 0 18 18" aria-hidden>
      <path fill="#4285F4" d="M17.6 9.2c0-.6-.1-1.3-.2-1.9H9v3.5h4.8a4.1 4.1 0 0 1-1.8 2.7v2.3h2.9c1.7-1.6 2.7-3.9 2.7-6.6Z" />
      <path fill="#34A853" d="M9 18c2.4 0 4.5-.8 6-2.2l-2.9-2.3c-.8.5-1.8.9-3.1.9-2.4 0-4.4-1.6-5.1-3.8H.9v2.4A9 9 0 0 0 9 18Z" />
      <path fill="#FBBC05" d="M3.9 10.7a5.4 5.4 0 0 1 0-3.4V4.9H.9a9 9 0 0 0 0 8.1l3-2.3Z" />
      <path fill="#EA4335" d="M9 3.6c1.3 0 2.5.5 3.4 1.3l2.6-2.6A9 9 0 0 0 .9 4.9l3 2.4C4.6 5.2 6.6 3.6 9 3.6Z" />
    </svg>
  );
}
