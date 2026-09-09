import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { useTranslation } from "react-i18next";
import { AuthLayout, RecoverForm } from "@/features/auth";
import { session } from "@/shared/api/session";
import { toast } from "@/shared/lib/toast";

export const Route = createFileRoute("/recover")({
  beforeLoad: () => {
    if (session.isAuthenticated()) throw redirect({ to: "/" });
  },
  component: RecoverPage,
});

function RecoverPage() {
  const navigate = useNavigate();
  const { t } = useTranslation();

  // Новый пароль токенов не выдаёт — дальше обычный вход, как и в старой
  // админке.
  return (
    <AuthLayout>
      <RecoverForm
        onDone={() => {
          toast.success(t("auth.recoverDone"));
          void navigate({ to: "/login" });
        }}
      />
    </AuthLayout>
  );
}
