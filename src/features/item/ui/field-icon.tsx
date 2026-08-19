import {
  IconAbc,
  IconAlignLeft,
  IconBarcode,
  IconBolt,
  IconBraces,
  IconCalendar,
  IconCalendarTime,
  IconCheckbox,
  IconCircleDot,
  IconClock,
  IconCode,
  IconHash,
  IconId,
  IconLink,
  IconList,
  IconLock,
  IconMail,
  IconMapPin,
  IconMath,
  IconMoodSmile,
  IconPalette,
  IconPaperclip,
  IconPhone,
  IconPhoto,
  IconPolygon,
  IconQrcode,
  IconWorld,
  type Icon as TablerIcon,
} from "@tabler/icons-react";
import { cellKind, type CellKind } from "../model/cell-kind";

/**
 * Иконка по типу поля. Одна на всё: заголовок колонки, чип фильтра
 * и список типов при создании поля должны показывать одно и то же —
 * иначе одно поле выглядит тремя разными сущностями.
 *
 * Ищется сначала по типу, потом по ВИДУ ячейки. Второй шаг важнее
 * первого: видов пятнадцать, типов сорок, и новый тип получает
 * осмысленную иконку сам, а не «Abc» — ровно так LINK и оказался
 * подписан как строка.
 */
const BY_KIND: Record<CellKind, TablerIcon> = {
  text: IconAbc,
  longtext: IconAlignLeft,
  number: IconHash,
  boolean: IconCheckbox,
  date: IconCalendar,
  datetime: IconCalendarTime,
  datetime_naive: IconCalendarTime,
  time: IconClock,
  status: IconCircleDot,
  multiselect: IconList,
  relation: IconLink,
  image: IconPhoto,
  file: IconPaperclip,
  color: IconPalette,
  icon: IconMoodSmile,
  map: IconMapPin,
  polygon: IconPolygon,
  json: IconBraces,
  formula: IconMath,
  qr: IconQrcode,
  barcode: IconBarcode,
  password: IconLock,
  link: IconWorld,
  button: IconBolt,
};

/** Типы, которые внутри своего вида всё же различаются на глаз. */
const BY_TYPE: Record<string, TablerIcon> = {
  EMAIL: IconMail,
  PHONE: IconPhone,
  INTERNATION_PHONE: IconPhone,
  UUID: IconId,
  RANDOM_UUID: IconId,
  RANDOM_TEXT: IconId,
  PRIMARY_KEY: IconId,
  INCREMENT_ID: IconId,
  INCREMENT_NUMBER: IconId,
  FORMULA: IconMath,
  FORMULA_FRONTEND: IconMath,
  // Тоже собранное значение, но строка, а не число.
  MANUAL_STRING: IconMath,
  CODE: IconCode,
  PROGRAMMING_LANGUAGE: IconCode,
  MAP: IconMapPin,
  PICK_LIST: IconList,
};

export function fieldIcon(type: string): TablerIcon {
  return BY_TYPE[type] ?? BY_KIND[cellKind(type)];
}
