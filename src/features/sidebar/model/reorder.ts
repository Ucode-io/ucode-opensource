import type { MenuNode } from "./types";

export type DropPosition = "before" | "after" | "inside";

/** Откуда тащим: узел и его уровень. Уровень знает только тот список, который загружен. */
export type DragSource = {
  node: MenuNode;
  parentId: string;
};

export type DropContext = {
  /** Пункт, над которым отпустили. */
  target: MenuNode;
  position: DropPosition;
  /** Соседи цели — то есть загруженный уровень, где лежит target. */
  targetSiblings: MenuNode[];
  /** Родитель этого уровня. */
  targetParentId: string;
  /**
   * Путь к уровню цели: id от корня до targetParentId включительно.
   * Нужен, чтобы не уронить папку внутрь собственного потомка.
   */
  targetPath: string[];
  /**
   * Дети целевой папки, если уровень уже загружен. Для position "inside".
   * undefined — папку ещё не раскрывали.
   */
  insideSiblings?: MenuNode[];
};

export type MovePlan = {
  parentId: string;
  /** Новый порядок соседей. Позиция в массиве и есть order. */
  siblings: MenuNode[];
  /** id пункта, если он сменил папку. */
  movedId?: string;
};

/**
 * Считает новый порядок после перетаскивания. Чистая функция: ни дерева,
 * ни запросов — поэтому проверяется тестом, а не кликами мышью.
 *
 * null означает «делать нечего»: вызывающий код просто ничего не отправляет.
 */
export function planMove(source: DragSource, drop: DropContext): MovePlan | null {
  const { node, parentId: fromParent } = source;
  const { target, position, targetSiblings, targetParentId, targetPath, insideSiblings } = drop;

  if (node.id === target.id) return null;

  /*
   * Папка внутрь собственного потомка. Новым родителем становится либо
   * targetParentId (before/after), либо сам target (inside) — и то и другое
   * лежит на targetPath. Если на этом пути встретился сам перетаскиваемый
   * пункт, ветка замкнётся в кольцо: она пропадёт из сайдбара целиком,
   * потому что от корня до неё больше не дойти, а вернуть можно только
   * запросом мимо интерфейса.
   */
  if (targetPath.includes(node.id)) return null;

  if (position === "inside") {
    // Внутрь можно только в то, что раскрывается.
    if (target.kind !== "group") return null;

    const current = (insideSiblings ?? []).filter((n) => n.id !== node.id);

    return {
      parentId: target.id,
      siblings: [...current, node],
      ...(fromParent === target.id ? {} : { movedId: node.id }),
    };
  }

  const rest = targetSiblings.filter((n) => n.id !== node.id);
  const overIndex = rest.findIndex((n) => n.id === target.id);
  if (overIndex === -1) return null;

  const siblings = [...rest];
  siblings.splice(position === "before" ? overIndex : overIndex + 1, 0, node);

  const sameParent = fromParent === targetParentId;
  const unchanged =
    sameParent &&
    targetSiblings.map((n) => n.id).join() === siblings.map((n) => n.id).join();

  // Порядок не изменился — запрос слать незачем.
  if (unchanged) return null;

  return {
    parentId: targetParentId,
    siblings,
    ...(sameParent ? {} : { movedId: node.id }),
  };
}
