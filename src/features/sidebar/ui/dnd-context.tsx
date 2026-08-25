import { createContext, useContext, useState, type DragEvent, type ReactNode } from "react";
import { useGlobalRight } from "@/features/auth";
import { useReorderMenus } from "../api/menus";
import {
  planMove,
  type DragSource,
  type DropContext,
  type DropPosition,
} from "../model/reorder";

type DropTarget = { id: string; position: DropPosition };

type Dnd = {
  /**
   * Разрешено ли роли менять порядок пунктов — глобальное право
   * `menu_drag`. Спрошено один раз здесь: строк на экране десятки,
   * а ответ у них общий.
   */
  enabled: boolean;
  source: DragSource | null;
  target: DropTarget | null;
  start: (source: DragSource) => void;
  end: () => void;
  hover: (target: DropTarget) => void;
  drop: (context: DropContext) => void;
};

const Context = createContext<Dnd | null>(null);

/**
 * Перетаскивание живёт над уровнями: пункт тащат из одного загруженного
 * уровня в другой, и оба должны знать общее состояние.
 *
 * Нативный HTML5 drag-and-drop, без библиотеки: задача узкая — переставить
 * соседей и перенести в папку. dnd-kit принёс бы 40 КБ ради этих двух действий.
 */
export function MenuDndProvider({ children }: { children: ReactNode }) {
  const [source, setSource] = useState<DragSource | null>(null);
  const [target, setTarget] = useState<DropTarget | null>(null);
  const reorder = useReorderMenus();
  // Порядок пунктов роли меняют не все: в старой админке это глобальное
  // право `menu_drag` (LayoutSidebar/index.jsx:163), и переключатель для
  // него есть в настройках роли.
  const canDrag = useGlobalRight("menu_drag");

  const end = () => {
    setSource(null);
    setTarget(null);
  };

  const value: Dnd = {
    enabled: canDrag,
    source,
    target,
    start: setSource,
    end,
    hover: (next) =>
      setTarget((current) =>
        current?.id === next.id && current.position === next.position ? current : next,
      ),
    drop: (context) => {
      if (!source) return end();
      const plan = planMove(source, context);
      end();
      // fromParentId нужен мутации, чтобы убрать пункт из кэша того
      // уровня, откуда его утащили.
      if (plan) reorder.mutate({ ...plan, fromParentId: source.parentId, node: source.node });
    },
  };

  return <Context.Provider value={value}>{children}</Context.Provider>;
}

export function useDnd(): Dnd {
  const value = useContext(Context);
  if (!value) throw new Error("MenuDndProvider отсутствует над деревом меню");
  return value;
}

/** Куда попадёт пункт, решает позиция курсора внутри строки. */
export function positionIn(event: DragEvent<HTMLElement>, canNest: boolean): DropPosition {
  const box = event.currentTarget.getBoundingClientRect();
  const offset = (event.clientY - box.top) / box.height;

  if (!canNest) return offset < 0.5 ? "before" : "after";
  if (offset < 0.25) return "before";
  if (offset > 0.75) return "after";
  return "inside";
}
