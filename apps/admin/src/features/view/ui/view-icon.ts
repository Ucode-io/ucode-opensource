import {
  IconCalendarWeek,
  IconChartPie4,
  IconClearAll,
  IconLayoutColumns,
  IconLayoutGrid,
  IconLayoutList,
  IconSitemap,
  IconTable,
  IconTablePlus,
  type Icon as TablerIcon,
} from "@tabler/icons-react";

/**
 * Значок типа view. Один на вкладки и на выбор типа в настройках.
 *
 * Значок здесь работает в 14px рядом с подписью, и в этом размере
 * выигрывает силуэт, а не подробность: значок из трёх линий читается,
 * значок из десяти превращается в пятно.
 *
 * Набор выбран заказчиком; здесь записано, что каждый значит, чтобы
 * следующая правка не свелась к «поменяю, вроде похоже»:
 *   CALENDAR  — неделя строкой. Не пустая рамка с засечкой: у голого
 *               `IconCalendar` внутри мелкая «1», которая в 14px
 *               становится кляксой.
 *   PIVOT     — таблица с плюсом, тот же значок, что у пункта меню
 *               типа PIVOT в сайдбаре (`sidebar/ui/MenuIcon`). Один
 *               смысл — один значок в обоих местах.
 *   TIMELINE  — три ступенчатые полосы: силуэт ленты, где у каждой
 *               строки свой отрезок во времени. Значка гантта у Tabler
 *               нет (проверено в 3.46, последней), а `IconTimeline`
 *               — ломаная линия, то есть график, а не лента.
 *   TREE      — карта узлов со связями.
 *   CHART     — сектор круга. Экран показывает восемь форм сразу,
 *               и значок отвечает «здесь диаграммы», а не называет
 *               одну из них.
 */
const ICONS: Record<string, TablerIcon> = {
  TABLE: IconTable,
  BOARD: IconLayoutColumns,
  CALENDAR: IconCalendarWeek,
  CHART: IconChartPie4,
  GRID: IconLayoutGrid,
  PIVOT: IconTablePlus,
  SECTION: IconLayoutList,
  TIMELINE: IconClearAll,
  TREE: IconSitemap,
};

export function viewIcon(type: string): TablerIcon {
  return ICONS[type] ?? IconTable;
}
