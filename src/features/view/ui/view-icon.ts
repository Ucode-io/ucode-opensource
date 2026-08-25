import {
  IconBinaryTree2,
  IconCalendarMonth,
  IconClearAll,
  IconLayoutColumns,
  IconLayoutGrid,
  IconLayoutList,
  IconSum,
  IconTable,
  type Icon as TablerIcon,
} from "@tabler/icons-react";

/**
 * Значок типа view. Один на вкладки и на выбор типа в настройках.
 *
 * Значок здесь работает в 14px рядом с подписью, и в этом размере
 * выигрывает силуэт, а не подробность: значок из трёх линий читается,
 * значок из десяти превращается в пятно. Второе требование — чтобы
 * восемь значков не путались МЕЖДУ СОБОЙ: рамка с сеткой внутри
 * годится ровно одному типу, и это TABLE.
 *
 * Поэтому:
 *   CALENDAR  — сетка месяца, а не пустая рамка с одной засечкой:
 *               у `IconCalendar` внутри мелкая «1», которая в 14px
 *               становится кляксой.
 *   PIVOT     — Σ: сводная не показывает строки, она их СЧИТАЕТ.
 *               Любая рамка с делениями читалась бы как TABLE или GRID,
 *               а прежний `IconChartArrowsVertical` — как «статистика»,
 *               то есть ни как что.
 *   TIMELINE  — три ступенчатые полосы: ровно силуэт ленты, где у каждой
 *               строки свой отрезок во времени. Значка гантта у Tabler
 *               нет (проверено в 3.46, последней), а прежний `IconTimeline`
 *               — ломаная линия, то есть график, а не лента.
 *   TREE      — дерево с узлами. `IconSitemap` — три прямоугольника
 *               со связями: в 14px это три пятна.
 */
const ICONS: Record<string, TablerIcon> = {
  TABLE: IconTable,
  BOARD: IconLayoutColumns,
  CALENDAR: IconCalendarMonth,
  GRID: IconLayoutGrid,
  PIVOT: IconSum,
  SECTION: IconLayoutList,
  TIMELINE: IconClearAll,
  TREE: IconBinaryTree2,
};

export function viewIcon(type: string): TablerIcon {
  return ICONS[type] ?? IconTable;
}
