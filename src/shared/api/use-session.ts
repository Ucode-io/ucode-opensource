import { useSyncExternalStore } from "react";
import { session } from "./session";

/**
 * Подписка на сессию.
 *
 * Сессия живёт вне React — это модуль с localStorage. Компонент, который
 * читает из неё projectId во время рендера, но не подписан, продолжит
 * рендериться со старым значением: после переключения проекта ключи
 * запросов не поменяются, и данные обновятся только после перезагрузки
 * страницы. Ровно это и происходило.
 *
 * Возвращает сам объект сессии: он стабильный, а перерисовку вызывает
 * смена версии.
 */
export function useSession() {
  useSyncExternalStore(session.subscribe, session.getVersion, session.getVersion);
  return session;
}
