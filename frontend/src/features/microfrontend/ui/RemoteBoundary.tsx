import { Component, type ReactNode } from "react";

/**
 * Граница ошибки вокруг ремоута.
 *
 * Без неё сбой в чужом коде — а он чужой и собран отдельно — снимает
 * с экрана всю админку: React размонтирует дерево до корня. Здесь же
 * падение остаётся внутри ремоута, и всё вокруг стоит на месте.
 *
 * Общая для пункта меню и для экрана входа: на входе цена падения выше
 * всего — без запасного пути в систему было бы не войти вовсе.
 */
export class RemoteBoundary extends Component<
  {
    children: ReactNode;
    /**
     * Функцией — когда текст зависит от причины: «ремоут новее админки»
     * человеку помогает, а «не загрузилось» на том же месте нет.
     */
    fallback: ReactNode | ((error: unknown) => ReactNode);
  },
  { failed: boolean; error: unknown }
> {
  override state = { failed: false, error: undefined as unknown };

  static getDerivedStateFromError(error: unknown) {
    return { failed: true, error };
  }

  override componentDidCatch(error: unknown) {
    // Причина нужна тому, кто собирал ремоут: несовпавшая версия,
    // отсутствующий `./Page`, CORS. В интерфейс её целиком не выносим —
    // читать её всё равно по стеку.
    console.error("microfrontend failed", error);
  }

  override render() {
    if (!this.state.failed) return this.props.children;

    const { fallback } = this.props;
    return typeof fallback === "function" ? fallback(this.state.error) : fallback;
  }
}
