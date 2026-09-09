import type { Icon as TablerIcon, IconProps } from "@tabler/icons-react";

/**
 * Обёртка над Tabler Icons. Задаёт наш размер и толщину линии в одном
 * месте: 16px и 1.6 вместо родных 24px и 2 — иначе иконки перевешивают
 * текст в плотном интерфейсе.
 *
 * Импортировать иконки поимённо (`import { IconPlus } from "@tabler/icons-react"`)
 * — пакет древовидно вытряхивается, в бандл попадают только использованные.
 */
export function Icon({
  as: Component,
  size = 16,
  className = "",
  ...props
}: IconProps & { as: TablerIcon; size?: number }) {
  return (
    <Component
      size={size}
      stroke={1.6}
      className={`shrink-0 ${className}`}
      aria-hidden
      {...props}
    />
  );
}
