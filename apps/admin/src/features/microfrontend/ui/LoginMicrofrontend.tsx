import { Suspense, lazy, useMemo, type ComponentType, type ReactNode } from "react";
import i18n from "@/shared/lib/i18n";
import { loginSubdomain, useLoginMicrofront } from "../api/login-microfront";
import { loadRemotePage, type LoginRemoteProps } from "../model/remote";
import { RemoteBoundary } from "./RemoteBoundary";
import { RemoteHost } from "./RemoteHost";

/**
 * Экран входа проекта: чужая форма логина вместо нашей.
 *
 * Привязка ищется по поддомену, на котором мы открыты, — так её и
 * заводят. Не нашлась, ещё едет, сборка не загрузилась или упала —
 * показывается `fallback`, то есть НАША форма.
 *
 * Запасной путь здесь важнее, чем где-либо ещё: у пункта меню
 * сломавшийся ремоут стоит одного экрана, а здесь — входа в систему
 * целиком. Старая админка запасного пути не имела: она подменяла весь
 * маршрут `/login` (`router/NewRouter.jsx:145`), и упавший ремоут
 * означал, что войти нечем.
 *
 * Пока привязка едет, показывается тоже наша форма, а не пустота:
 * запрос неавторизованный и обычно быстрый, а мелькание «загрузка»
 * перед формой входа выглядит хуже, чем форма, поверх которой через
 * миг встала чужая.
 */
export function LoginMicrofrontend({
  onLogin,
  fallback,
}: {
  /** Вход теми данными, что собрал ремоут. Наш, не его. */
  onLogin: (credentials: { username: string; password: string }) => Promise<void>;
  /** Наша форма входа. Показывается всегда, когда ремоута нет. */
  fallback: ReactNode;
}) {
  const subdomain = loginSubdomain();
  const { binding } = useLoginMicrofront(subdomain);

  const url = binding?.url ?? "";
  const id = binding?.microfrontId ?? "";

  const Page = useMemo(() => {
    if (!id || !url) return null;

    // Две ветки те же, что у пункта меню (см. MicrofrontendPage):
    // новый ремоут — наш компонент, старый — свой корень React 18.
    return lazy(async () => {
      const { page, meta } = await loadRemotePage(id, url);

      if (meta) return { default: page as ComponentType<LoginRemoteProps> };
      return { default: (props: LoginRemoteProps) => <RemoteHost page={page} props={props} /> };
    });
  }, [id, url]);

  if (!Page) return fallback;

  return (
    <RemoteBoundary key={`${id}:${url}`} fallback={fallback}>
      <Suspense fallback={fallback}>
        <Page loginAction={onLogin} i18n={i18n} />
      </Suspense>
    </RemoteBoundary>
  );
}
