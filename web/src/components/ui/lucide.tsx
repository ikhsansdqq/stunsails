// Drop-in for "lucide-react" across StunSails, as in freelance-dashboard: the same names
// and props, but each icon becomes the animated version (@animateicons/react) once
// that set has loaded after start-up (see ./icons.tsx), and plays when the button,
// link, menu item, tab or row it sits in is hovered or focused. Until then it is the
// plain lucide-react glyph, so first paint is unchanged.
//
// Import icons from here, never from "lucide-react". A new icon: add it below and
// its animated name to ./icons-animated.ts.
import { forwardRef, useEffect, useRef, type CSSProperties, type ForwardRefExoticComponent, type RefAttributes } from "react";
import type { IconHandle } from "@animateicons/react";
import {
  Activity as StaticActivity,
  ArrowDown as StaticArrowDown,
  ArrowDownToLine as StaticArrowDownToLine,
  ArrowUp as StaticArrowUp,
  Ban as StaticBan,
  Bell as StaticBell,
  Box as StaticBox,
  ChevronFirst as StaticChevronFirst,
  ChevronLast as StaticChevronLast,
  CornerDownLeft as StaticCornerDownLeft,
  MapPin as StaticMapPin,
  Boxes as StaticBoxes,
  CalendarClock as StaticCalendarClock,
  Check as StaticCheck,
  ChevronDown as StaticChevronDown,
  ChevronLeft as StaticChevronLeft,
  ChevronRight as StaticChevronRight,
  ChevronsUpDown as StaticChevronsUpDown,
  CircleAlert as StaticCircleAlert,
  CircleCheck as StaticCircleCheck,
  CirclePause as StaticCirclePause,
  CirclePlay as StaticCirclePlay,
  CircleStop as StaticCircleStop,
  CircleX as StaticCircleX,
  Clock as StaticClock,
  CloudDownload as StaticCloudDownload,
  Command as StaticCommand,
  Copy as StaticCopy,
  Cpu as StaticCpu,
  Download as StaticDownload,
  Ellipsis as StaticEllipsis,
  EllipsisVertical as StaticEllipsisVertical,
  Eraser as StaticEraser,
  ExternalLink as StaticExternalLink,
  Eye as StaticEye,
  EyeOff as StaticEyeOff,
  FileSearch as StaticFileSearch,
  Gauge as StaticGauge,
  Globe as StaticGlobe,
  HardDrive as StaticHardDrive,
  History as StaticHistory,
  Image as StaticImage,
  Info as StaticInfo,
  KeyRound as StaticKeyRound,
  LayoutDashboard as StaticLayoutDashboard,
  LayoutGrid as StaticLayoutGrid,
  ListChecks as StaticListChecks,
  LoaderCircle as StaticLoaderCircle,
  LogOut as StaticLogOut,
  Mail as StaticMail,
  Menu as StaticMenu,
  Minus as StaticMinus,
  Moon as StaticMoon,
  Network as StaticNetwork,
  Package as StaticPackage,
  Pause as StaticPause,
  Play as StaticPlay,
  Plus as StaticPlus,
  Power as StaticPower,
  RefreshCw as StaticRefreshCw,
  RotateCw as StaticRotateCw,
  ScrollText as StaticScrollText,
  Search as StaticSearch,
  Send as StaticSend,
  Server as StaticServer,
  Settings as StaticSettings,
  ShieldAlert as StaticShieldAlert,
  Siren as StaticSiren,
  Sun as StaticSun,
  Terminal as StaticTerminal,
  Trash2 as StaticTrash2,
  TriangleAlert as StaticTriangleAlert,
  Webhook as StaticWebhook,
  Wrench as StaticWrench,
  X as StaticX,
  Zap as StaticZap,
  type LucideIcon as StaticIcon,
  type LucideProps,
} from "lucide-react";
import { cn } from "@/lib/utils";
import { useAnimatedIcons, type AnimatedName } from "./icons";

export type { LucideProps };
export type LucideIcon = ForwardRefExoticComponent<Omit<LucideProps, "ref"> & RefAttributes<SVGSVGElement>>;

/** The icon's size in px from a size utility (size-4, size-[18px], h-5 w-5), else undefined. */
function sizeOf(className: string | undefined, size: LucideProps["size"]) {
  if (typeof size === "number") return size;
  if (typeof size === "string" && /^\d+$/.test(size)) return Number(size);
  const c = className ?? "";
  const unit = c.match(/(?:^|\s)(?:size|h)-(\d+(?:\.\d+)?)(?=\s|$)/);
  if (unit) return Number(unit[1]) * 4;
  const px = c.match(/(?:^|\s)(?:size|h)-\[(\d+(?:\.\d+)?)px\](?=\s|$)/);
  return px ? Number(px[1]) : undefined;
}

/** What hovering or focusing plays the icon: the control or row it sits in. */
const host = "a, button, label, summary, tr, li, [role=menuitem], [role=option], [role=tab], [role=radio], [role=button], [data-icon-host]";

function Live({ name, className, size, style, ...rest }: { name: AnimatedName; className?: string; size?: LucideProps["size"]; style?: CSSProperties; "aria-hidden"?: LucideProps["aria-hidden"]; "aria-label"?: string }) {
  const lib = useAnimatedIcons()!;
  const Real = lib[name] as ForwardRefExoticComponent<{ size?: number; "aria-hidden"?: boolean } & RefAttributes<IconHandle>>;
  const handle = useRef<IconHandle>(null);
  const box = useRef<HTMLSpanElement>(null);
  useEffect(() => {
    const el = box.current?.parentElement?.closest<HTMLElement>(host);
    if (!el) return;
    const play = () => handle.current?.startAnimation();
    const stop = () => handle.current?.stopAnimation();
    el.addEventListener("pointerenter", play);
    el.addEventListener("focusin", play);
    el.addEventListener("pointerleave", stop);
    el.addEventListener("focusout", stop);
    return () => {
      el.removeEventListener("pointerenter", play);
      el.removeEventListener("focusin", play);
      el.removeEventListener("pointerleave", stop);
      el.removeEventListener("focusout", stop);
    };
  }, []);
  const label = rest["aria-label"];
  // btn-icon: buttons treat it as their leading icon, as they do a bare svg (button.tsx, .btn-arrow).
  return (
    <span ref={box} className={cn("btn-icon inline-flex shrink-0 items-center justify-center", className)} style={style} {...(label ? { role: "img", "aria-label": label } : { "aria-hidden": true })}>
      <Real ref={handle} size={sizeOf(className, size) ?? 24} aria-hidden />
    </span>
  );
}

function drop(name: AnimatedName | null, Static: StaticIcon): LucideIcon {
  const Icon = forwardRef<SVGSVGElement, Omit<LucideProps, "ref">>(function Icon(props, ref) {
    const lib = useAnimatedIcons();
    // Spinners keep spinning: the static glyph with animate-spin, not a hover animation.
    if (!name || !lib || props.className?.includes("animate-spin")) return <Static ref={ref} {...props} />;
    return <Live name={name} className={props.className} size={props.size} style={props.style} aria-hidden={props["aria-hidden"]} aria-label={props["aria-label"]} />;
  });
  Icon.displayName = Static.displayName;
  return Icon;
}

export const Activity = drop("ActivityIcon", StaticActivity);
export const ArrowDown = drop("ArrowDownIcon", StaticArrowDown);
export const ArrowDownToLine = drop("ArrowDownToLineIcon", StaticArrowDownToLine);
export const ArrowUp = drop("ArrowUpIcon", StaticArrowUp);
export const Ban = drop("BanIcon", StaticBan);
export const Bell = drop("BellIcon", StaticBell);
export const Box = drop("BoxIcon", StaticBox);
export const ChevronFirst = drop("ChevronFirstIcon", StaticChevronFirst);
export const ChevronLast = drop("ChevronLastIcon", StaticChevronLast);
export const CornerDownLeft = drop("CornerDownLeftIcon", StaticCornerDownLeft);
export const MapPin = drop("MapPinIcon", StaticMapPin);
export const Boxes = drop("BoxesIcon", StaticBoxes);
export const CalendarClock = drop("CalendarClockIcon", StaticCalendarClock);
export const Check = drop("CheckIcon", StaticCheck);
export const ChevronDown = drop("ChevronDownIcon", StaticChevronDown);
export const ChevronLeft = drop("ChevronLeftIcon", StaticChevronLeft);
export const ChevronRight = drop("ChevronRightIcon", StaticChevronRight);
export const ChevronsUpDown = drop("ChevronsUpDownIcon", StaticChevronsUpDown);
export const CircleAlert = drop("CircleAlertIcon", StaticCircleAlert);
export const CircleCheck = drop("CircleCheckIcon", StaticCircleCheck);
export const CirclePause = drop("CirclePauseIcon", StaticCirclePause);
export const CirclePlay = drop("CirclePlayIcon", StaticCirclePlay);
export const CircleStop = drop("CircleStopIcon", StaticCircleStop);
export const CircleX = drop("CircleXIcon", StaticCircleX);
export const Clock = drop("ClockIcon", StaticClock);
export const CloudDownload = drop("CloudDownloadIcon", StaticCloudDownload);
export const Command = drop("CommandIcon", StaticCommand);
export const Copy = drop("CopyIcon", StaticCopy);
export const Cpu = drop("CpuIcon", StaticCpu);
export const Download = drop("DownloadIcon", StaticDownload);
export const Ellipsis = drop("EllipsisIcon", StaticEllipsis);
export const EllipsisVertical = drop("EllipsisVerticalIcon", StaticEllipsisVertical);
export const Eraser = drop("EraserIcon", StaticEraser);
export const ExternalLink = drop("ExternalLinkIcon", StaticExternalLink);
export const Eye = drop("EyeIcon", StaticEye);
export const EyeOff = drop("EyeOffIcon", StaticEyeOff);
export const FileSearch = drop("FileSearchIcon", StaticFileSearch);
export const Gauge = drop("GaugeIcon", StaticGauge);
export const Globe = drop("GlobeIcon", StaticGlobe);
export const HardDrive = drop("HardDriveIcon", StaticHardDrive);
export const History = drop("HistoryIcon", StaticHistory);
export const Image = drop("ImageIcon", StaticImage);
export const Info = drop("InfoIcon", StaticInfo);
export const KeyRound = drop("KeyRoundIcon", StaticKeyRound);
export const LayoutDashboard = drop("LayoutDashboardIcon", StaticLayoutDashboard);
export const LayoutGrid = drop("LayoutGridIcon", StaticLayoutGrid);
export const ListChecks = drop("ListChecksIcon", StaticListChecks);
export const LoaderCircle = drop("LoaderCircleIcon", StaticLoaderCircle);
export const LogOut = drop("LogOutIcon", StaticLogOut);
export const Mail = drop("MailIcon", StaticMail);
export const Menu = drop("MenuIcon", StaticMenu);
export const Minus = drop("MinusIcon", StaticMinus);
export const Moon = drop("MoonIcon", StaticMoon);
export const Network = drop("NetworkIcon", StaticNetwork);
export const Package = drop("PackageIcon", StaticPackage);
export const Pause = drop("PauseIcon", StaticPause);
export const Play = drop("PlayIcon", StaticPlay);
export const Plus = drop("PlusIcon", StaticPlus);
export const Power = drop("PowerIcon", StaticPower);
export const RefreshCw = drop("RefreshCwIcon", StaticRefreshCw);
export const RotateCw = drop("RotateCwIcon", StaticRotateCw);
export const ScrollText = drop("ScrollTextIcon", StaticScrollText);
export const Search = drop("SearchIcon", StaticSearch);
export const Send = drop("SendIcon", StaticSend);
export const Server = drop("ServerIcon", StaticServer);
export const Settings = drop("SettingsIcon", StaticSettings);
export const ShieldAlert = drop("ShieldAlertIcon", StaticShieldAlert);
export const Siren = drop("SirenIcon", StaticSiren);
export const Sun = drop("SunIcon", StaticSun);
export const Terminal = drop("TerminalIcon", StaticTerminal);
export const Trash2 = drop("Trash2Icon", StaticTrash2);
export const TriangleAlert = drop("TriangleAlertIcon", StaticTriangleAlert);
export const Webhook = drop("WebhookIcon", StaticWebhook);
export const Wrench = drop("WrenchIcon", StaticWrench);
export const X = drop("XIcon", StaticX);
export const Zap = drop("ZapIcon", StaticZap);
