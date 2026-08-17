import {
  IconCalendar,
  IconChartArrowsVertical,
  IconLayoutColumns,
  IconLayoutGrid,
  IconLayoutList,
  IconTimeline,
  IconSitemap,
  IconTable,
  type Icon as TablerIcon,
} from "@tabler/icons-react";

/** Значок типа view. Один на вкладки и на выбор типа в настройках. */
const ICONS: Record<string, TablerIcon> = {
  TABLE: IconTable,
  BOARD: IconLayoutColumns,
  CALENDAR: IconCalendar,
  GRID: IconLayoutGrid,
  PIVOT: IconChartArrowsVertical,
  SECTION: IconLayoutList,
  TIMELINE: IconTimeline,
  TREE: IconSitemap,
};

export function viewIcon(type: string): TablerIcon {
  return ICONS[type] ?? IconTable;
}
