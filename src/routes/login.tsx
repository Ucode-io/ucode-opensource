import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { AuthLayout, LoginForm } from "@/features/auth";
import { session } from "@/shared/api/session";

export const Route = createFileRoute("/login")({
  beforeLoad: () => {
    if (session.isAuthenticated()) throw redirect({ to: "/" });
  },
  component: LoginPage,
});

function LoginPage() {
  const navigate = useNavigate();

  return (
    <AuthLayout>
      <LoginForm onSuccess={() => void navigate({ to: "/" })} />
    </AuthLayout>
  );
}
