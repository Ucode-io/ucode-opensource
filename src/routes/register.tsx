import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { AuthLayout, RegisterForm } from "@/features/auth";
import { session } from "@/shared/api/session";

export const Route = createFileRoute("/register")({
  beforeLoad: () => {
    if (session.isAuthenticated()) throw redirect({ to: "/" });
  },
  component: RegisterPage,
});

function RegisterPage() {
  const navigate = useNavigate();

  // Регистрация не выдаёт токены — она создаёт компанию. Дальше обычный вход.
  return (
    <AuthLayout>
      <RegisterForm onSuccess={() => void navigate({ to: "/login" })} />
    </AuthLayout>
  );
}
