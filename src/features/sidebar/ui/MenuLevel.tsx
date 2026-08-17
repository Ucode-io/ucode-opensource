import { type DragEvent } from "react";
import { Link } from "@tanstack/react-router";
import { useUi } from "@/shared/lib/ui-store";
import { useMenuChildren } from "../api/menus";
import type { MenuNode } from "../model/types";
import { positionIn, useDnd } from "./dnd-context";
import { Chevron, MenuIcon } from "./MenuIcon";
import { MenuRowActions } from "./MenuRowActions";

/**
 * Обёртка строки: держит перетаскивание, наведение и кнопку действий.
 * Подсветка живёт здесь, а не на ссылке, иначе активный пункт окрашен
 * не целиком — место под «⋮» остаётся незакрашенным.
 *
 * has-[.row-active] ловит класс, который роутер вешает на активную ссылку.
 */
const row =
  "group/row relative flex h-8 w-full items-center rounded-md pr-1 transition-colors hover:bg-surface-hover has-[.row-active]:bg-surface has-[.row-active]:shadow-raised";

/** Сама ссылка или кнопка раскрытия — занимает всю строку, кроме «⋮». */
const inner_row =
  "flex h-8 min-w-0 flex-1 items-center gap-2 rounded-md text-left text-sm text-fg-muted";

/** Активный пункт: класс ловится обёрткой через has-[]. */
const activeRow = "row-active text-fg font-medium";

/**
 * Один уровень меню. Бэкенд отдаёт меню по одному уровню за запрос
 * (parent_id обязателен), поэтому уровень — это и компонент, и единица
 * загрузки: запрос уходит, когда папку раскрыли.
 *
 * path — id от корня до родителя этого уровня включительно. Он же задаёт
 * и отступ (длина), и родителя (последний элемент), и защиту от переноса
 * папки внутрь собственного потомка.
 */
export function MenuLevel({ items, path }: { items: MenuNode[]; path: string[] }) {
  return (
    <ul className="flex flex-col gap-0.5">
      {items.map((node) => (
        <li key={node.id}>
          <MenuRow node={node} siblings={items} path={path} />
        </li>
      ))}
    </ul>
  );
}

function MenuRow({
  node,
  siblings,
  path,
}: {
  node: MenuNode;
  siblings: MenuNode[];
  path: string[];
}) {
  const parentId = path[path.length - 1] ?? "";
  const depth = path.length - 1;
  const dnd = useDnd();
  const expandable = node.kind === "group";
  /*
   * Раскрытие переживает перезагрузку — оно в ui-store (см. expandedMenus).
   * Подписка селектором, а не всем стором: строк меню на экране десятки,
   * и правка ширины сайдбара перерисовывала бы каждую.
   */
  const open = useUi((state) => state.expandedMenus.includes(node.id));
  const toggleMenu = useUi((state) => state.toggleMenu);

  // Запрос уходит только когда папку раскрыли.
  const children = useMenuChildren(node.id, expandable && open);

  const dragging = dnd.source?.node.id === node.id;
  const hit = dnd.target?.id === node.id ? dnd.target.position : null;

  // Обработчики перетаскивания живут на обёртке, а не на ссылке: внутри
  // строки есть вторая кнопка, и тащить нужно строку целиком.
  const dragHandlers = {
    draggable: true,
    onDragStart: (event: DragEvent<HTMLElement>) => {
      dnd.start({ node, parentId });
      event.dataTransfer.effectAllowed = "move";
      // Без данных Firefox не начинает перетаскивание.
      event.dataTransfer.setData("text/plain", node.id);
      // Призрак — сама строка целиком, взятая под курсором.
      event.dataTransfer.setDragImage(event.currentTarget, 12, 16);
    },
    onDragEnd: dnd.end,
    onDragOver: (event: DragEvent<HTMLElement>) => {
      // Без preventDefault браузер показывает «сюда нельзя» — а подсветка
      // не зажигается, и рамка не обещает переноса, которого не будет.
      if (!dnd.source || dnd.source.node.id === node.id) return;
      if (path.includes(dnd.source.node.id)) return;
      event.preventDefault();
      event.dataTransfer.dropEffect = "move";
      dnd.hover({ id: node.id, position: positionIn(event, expandable) });
    },
    onDrop: (event: DragEvent<HTMLElement>) => {
      event.preventDefault();
      event.stopPropagation();
      dnd.drop({
        target: node,
        position: positionIn(event, expandable),
        targetSiblings: siblings,
        targetParentId: parentId,
        targetPath: path,
        ...(open ? { insideSiblings: children.items } : {}),
      });
    },
  };

  // Отступ вложенности инлайном: Tailwind не собирает классы из строк.
  const style = { paddingLeft: `${8 + depth * 14}px` };

  /**
   * У раскрываемых пунктов шеврон занимает место иконки при наведении:
   * иконка гаснет, стрелка проявляется в том же слоте. Так строка
   * не дёргается и не тратит второй слот справа.
   */
  const inner = (
    <>
      <span className="relative grid size-4 shrink-0 place-items-center">
        <span className={expandable ? "transition-opacity group-hover/row:opacity-0" : ""}>
          <MenuIcon name={node.icon} type={node.type} />
        </span>

        {expandable && (
          <span className="absolute opacity-0 transition-opacity group-hover/row:opacity-100">
            <Chevron open={open} />
          </span>
        )}
      </span>

      <span className="flex-1 truncate">{node.label}</span>
    </>
  );

  // draggable={false} обязателен: ссылки перетаскиваются браузером сами,
  // и вместо строки меню он тащит URL — с собственным призраком поверх
  // нашего. Тащит только обёртка.
  const navigable = expandable ? (
    <button
      type="button"
      onClick={() => toggleMenu(node.id)}
      aria-expanded={open}
      className={inner_row}
      style={style}
    >
      {inner}
    </button>
  ) : node.kind === "link" && node.href ? (
    <a
      href={node.href}
      target="_blank"
      rel="noreferrer"
      draggable={false}
      className={inner_row}
      style={style}
    >
      {inner}
    </a>
  ) : (
    <Link
      to="/m/$menuId"
      params={{ menuId: node.id }}
      draggable={false}
      className={inner_row}
      style={style}
      activeProps={{ className: `${inner_row} ${activeRow}` }}
    >
      {inner}
    </Link>
  );

  return (
    <>
      <div
        className={`${row} ${dragging ? "opacity-40" : ""} ${
          hit === "inside" ? "bg-accent-subtle ring-1 ring-accent ring-inset" : ""
        }`}
        {...dragHandlers}
      >
        {(hit === "before" || hit === "after") && (
          <span
            aria-hidden
            className={`absolute inset-x-1 h-0.5 rounded-full bg-accent ${
              hit === "before" ? "-top-px" : "-bottom-px"
            }`}
          />
        )}

        {navigable}
        <MenuRowActions node={node} isAdmin={false} />
      </div>

      {expandable && open && (
        <>
          {children.isLoading && <LevelSkeleton depth={depth + 1} />}
          <MenuLevel items={children.items} path={[...path, node.id]} />
        </>
      )}
    </>
  );
}

function LevelSkeleton({ depth }: { depth: number }) {
  return (
    <div className="flex flex-col gap-0.5" aria-hidden>
      {[60, 45].map((width, index) => (
        <div
          key={index}
          className="flex h-8 items-center"
          style={{ paddingLeft: `${8 + depth * 14}px` }}
        >
          <div className="h-3 rounded-sm bg-surface-active" style={{ width: `${width}%` }} />
        </div>
      ))}
    </div>
  );
}
