import {
  IconApi,
  IconBucket,
  IconCalendar,
  IconChartBar,
  IconChevronDown,
  IconFile,
  IconFolder,
  IconLayoutGrid,
  IconLink,
  IconSettings,
  IconTable,
  IconTablePlus,
  IconUsers,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import { DynamicIcon } from "@/shared/ui/dynamic-icon";
import { Icon } from "@/shared/ui/icon";
import { kindOf } from "../model/types";

/**
 * Иконка пункта меню.
 *
 * Сначала пробуем ту, что задал бэкенд (Iconify, URL или файл в CDN),
 * и только если её нет или она не загрузилась — рисуем свою по типу
 * пункта. Строка меню не остаётся пустой ни в один момент.
 */

/** Тип пункта → иконка по умолчанию. Покрывает все типы бэкенда. */
const byType: Record<string, TablerIcon> = {
  FOLDER: IconFolder,
  WIKI_FOLDER: IconFolder,
  MINIO_FOLDER: IconBucket,
  TABLE: IconTable,
  LINK: IconLink,
  MICROFRONTEND: IconLayoutGrid,
  PIVOT: IconTablePlus,
  REST: IconApi,
  USER: IconUsers,
  WEBPAGE: IconFile,
  WIKI: IconFile,
};

/** Популярные имена, которые присылает бэкенд, — на случай отказа CDN. */
const byName: Record<string, TablerIcon> = {
  settings: IconSettings,
  chart: IconChartBar,
  calendar: IconCalendar,
  table: IconTable,
  folder: IconFolder,
  users: IconUsers,
};

export function MenuIcon({ name, type }: { name: string; type: string }) {
  return <DynamicIcon name={name} fallback={<BuiltinIcon name={name} type={type} />} />;
}

function BuiltinIcon({ name, type }: { name: string; type: string }) {
  const fallback = kindOf(type) === "group" ? IconFolder : IconTable;
  const component = byName[name] ?? byType[type] ?? fallback;

  return <Icon as={component} />;
}

export function Chevron({ open }: { open: boolean }) {
  return (
    <Icon
      as={IconChevronDown}
      size={14}
      className={`transition-transform ${open ? "" : "-rotate-90"}`}
    />
  );
}
