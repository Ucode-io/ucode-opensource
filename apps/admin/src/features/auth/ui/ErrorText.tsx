import { useTranslation } from "react-i18next";
import { ApiError } from "@/shared/api/client";

export function ErrorText({ error }: { error: Error }) {
  const { t } = useTranslation();

  // Бэкенд кладёт причину в конверт строкой — клиент её уже развернул.
  const fromServer = error instanceof ApiError && typeof error.body === "string" ? error.body : null;

  // 401 при входе — это не «сессия истекла», а неверная пара логин/пароль.
  const message =
    error instanceof ApiError && error.status === 401
      ? t("auth.invalidCredentials")
      : (fromServer ?? error.message);

  return (
    <p role="alert" className="rounded-md bg-danger-subtle px-3 py-2 text-sm text-danger">
      {message}
    </p>
  );
}
