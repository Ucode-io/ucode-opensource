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
  { children: ReactNode; fallback: ReactNode },
  { failed: boolean }
> {
  override state = { failed: false };

  static getDerivedStateFromError() {
    return { failed: true };
  }

  override componentDidCatch(error: unknown) {
    // Причина нужна тому, кто собирал ремоут: несовпавшая версия,
    // отсутствующий `./Page`, CORS. В интерфейс её не выносим —
    // читать её всё равно по стеку.
    console.error("microfrontend failed", error);
  }

  override render() {
    return this.state.failed ? this.props.fallback : this.props.children;
  }
}
