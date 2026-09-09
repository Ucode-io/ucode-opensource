import { createFileRoute, redirect, useNavigate } from "@tanstack/react-router";
import { AuthLayout, LoginForm, useLogin } from "@/features/auth";
import { LoginMicrofrontend } from "@/features/microfrontend";
import { session } from "@/shared/api/session";
import i18n from "@/shared/lib/i18n";

export const Route = createFileRoute("/login")({
  beforeLoad: () => {
    if (session.isAuthenticated()) throw redirect({ to: "/" });
  },
  component: LoginPage,
});

function LoginPage() {
  const navigate = useNavigate();
  const login = useLogin();

  const done = () => void navigate({ to: "/" });

  /*
   * Проект может подменить экран входа своим микрофронтендом — тогда
   * наша форма остаётся запасным путём: её показывает `fallback`,
   * когда привязки нет, сборка не приехала или чужой код упал.
   */
  return (
    <LoginMicrofrontend
      onLogin={async (credentials) => {
        const result = await login.mutateAsync(credentials);

        if (result.kind === "session") return done();

        /*
         * Логин может потребовать второго шага — выбора connection'а
         * из списка. Чужой экран входа о таком шаге не знает и своей
         * формы для него не имеет, так что вызов отклоняется: ремоут
         * покажет отказ, а войти можно нашей формой по прямой ссылке.
         *
         * Ошибка, а не тихий возврат: «нажал войти и ничего» читается
         * как поломка.
         */
        throw new Error(i18n.t("microfrontend.loginNeedsChoice"));
      }}
      fallback={
        <AuthLayout>
          <LoginForm onSuccess={done} />
        </AuthLayout>
      }
    />
  );
}
