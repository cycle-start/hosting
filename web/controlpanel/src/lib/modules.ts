import {
  Globe,
  Database,
  Waypoints,
  Mail,
  Zap,
  HardDrive,
  Key,
  Archive,
  ShieldCheck,
} from "lucide-react";
import type { LucideIcon } from "lucide-react";

export const MODULE_META: Record<
  string,
  { icon: LucideIcon; labelKey: string; route: string }
> = {
  webroots: { icon: Globe, labelKey: "nav.webroots", route: "webroots" },
  databases: { icon: Database, labelKey: "nav.databases", route: "databases" },
  dns: { icon: Waypoints, labelKey: "nav.domains", route: "dns" },
  email: { icon: Mail, labelKey: "nav.email", route: "email" },
  valkey: { icon: Zap, labelKey: "nav.valkey", route: "valkey" },
  s3: { icon: HardDrive, labelKey: "nav.s3Storage", route: "s3" },
  wireguard: { icon: ShieldCheck, labelKey: "nav.wireguard", route: "wireguard" },
  ssh_keys: { icon: Key, labelKey: "nav.sshKeys", route: "ssh-keys" },
  backups: { icon: Archive, labelKey: "nav.backups", route: "backups" },
};
