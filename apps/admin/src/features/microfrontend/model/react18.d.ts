/**
 * Вторая копия React — та, на которой работают ремоуты. Лежит
 * отдельным пакетом в `vendor/react18` (почему не псевдонимом —
 * в его README), и типов у неё нет: `@types/react` в проекте один
 * и описывает React 19.
 *
 * Объявляем ровно то, чем пользуемся: два CJS-объекта уезжают
 * в общую область федерации как `unknown`, и три функции нужны нам
 * самим. Пропсы ремоута наш TypeScript всё равно не знает — договор
 * держится на `RemotePageProps`.
 */
declare module "react18/react" {
  const react: {
    createElement: (type: unknown, props?: object) => unknown;
    /** Версией отмечаем модуль в общей области — берём её у самого React. */
    version: string;
  };
  export default react;
}

declare module "react18/dom" {
  const reactDom: unknown;
  export default reactDom;
}

declare module "react18/client" {
  const client: {
    createRoot: (container: Element) => { render(node: unknown): void; unmount(): void };
  };
  export default client;
}
