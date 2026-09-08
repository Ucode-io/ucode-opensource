import { useEffect, useRef, useState } from "react";
import { createRemoteRoot, type RemoteComponent, type RemoteRoot } from "../model/remote";
import { containRemoteStyles } from "../model/remote-styles";

/**
 * Место ремоута на нашей странице.
 *
 * Ремоут рисует не наш React, а его собственный (React 18, см.
 * `model/remote.ts`), поэтому чужому дереву нужен свой узел DOM —
 * тот, в который наш React 19 не заглядывает. Наш рисует пустой `div`
 * и больше к нему не возвращается; узел для ремоута создаётся внутри
 * него руками и руками же убирается. Так два дерева не спорят за одни
 * и те же элементы.
 *
 * Внешний `div` — наш якорь с `display: contents`: своей коробки он не
 * заводит и в раскладку не вмешивается. Внутренний, тот что для ремоута,
 * наоборот, коробка со своей прокруткой — почему, написано ниже.
 */
export function RemoteHost({ page, props }: { page: RemoteComponent; props: object }) {
  const anchor = useRef<HTMLDivElement>(null);
  const [root, setRoot] = useState<RemoteRoot | null>(null);

  useEffect(() => {
    const parent = anchor.current;
    if (!parent) return;

    const node = parent.appendChild(document.createElement("div"));
    /*
     * Своя прокручиваемая коробка, а не `display: contents`.
     *
     * Сначала было `contents` — чтобы разметку ремоута раскладывал тот
     * же контейнер, что и всё вокруг, как в старой админке. Но у нашей
     * области содержимого высота ограничена и стоит `overflow: clip`,
     * а чужой экран выше: у живого микрофронтенда так срезало пагинацию,
     * и добраться до неё было нечем.
     *
     * Чужому дереву нужна своя высота и своя прокрутка — тогда оно
     * прокручивается внутри отведённого места, а наша шапка и сайдбар
     * остаются на месте.
     */
    node.className = "flex min-h-0 flex-1 flex-col overflow-auto";

    /*
     * Чужой CSS приходит вместе с ремоутом и по умолчанию побеждает
     * наш — не по весу, а потому что он вне слоёв каскада, а наши
     * утилиты в `@layer utilities`. Запираем его в этом узле,
     * см. model/remote-styles.
     */
    const releaseStyles = containRemoteStyles(node);

    let created: RemoteRoot | undefined;
    let live = true;

    void createRemoteRoot(node).then((remote) => {
      created = remote;
      if (live) setRoot(remote);
      else remote.unmount();
    });

    return () => {
      live = false;
      /*
       * Микротаск, а не сразу: React 18 ругается на синхронный `unmount`
       * посреди коммита React 19, а мы именно там и находимся — это
       * уборка эффекта. Узел убираем после ремоута, чтобы чужое дерево
       * успело отработать свои размонтирования на живом DOM.
       */
      releaseStyles();
      queueMicrotask(() => {
        created?.unmount();
        node.remove();
      });
    };
  }, []);

  // Без списка зависимостей: пропсы — свежесобранный объект на каждый
  // заход, и сравнивать его нечем. Перерисовка чужого дерева стоит
  // дешевле, чем мемоизация всего, что в него уезжает.
  useEffect(() => {
    root?.render(page, props);
  });

  return <div ref={anchor} className="contents" />;
}
