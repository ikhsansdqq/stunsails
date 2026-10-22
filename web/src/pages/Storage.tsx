import { useState } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { Link, useLocation } from "wouter";
import { toast } from "sonner";
import { Boxes, Copy, HardDrive, Network as NetworkIcon, RefreshCw, Trash2 } from "@/components/ui/lucide";
import { api, ApiError, type Bucket, type Image, type Network, type PruneKind, type Storage, type Volume } from "@/lib/api";
import { ago, formatBytes, shortId } from "@/lib/format";
import { cn } from "@/lib/utils";
import { PageHeader } from "@/components/Layout";
import { Notice } from "@/components/Notice";
import { Button } from "@/components/ui/button";
import { Section } from "@/components/ui/card";
import { Chip } from "@/components/ui/chip";
import { DataTable, columnsFor, type RowSelectionState } from "@/components/ui/data-table";
import { Block } from "@/components/ui/block";
import { RowMenu } from "@/components/ui/row-menu";
import { BulkBar, selectColumn } from "@/components/bulk";
import { copy } from "@/components/ContainerActions";
import { Segmented } from "@/components/ui/segmented";
import { Loading, SectionSkeleton } from "@/components/ui/skeleton";
import { useRun } from "@/components/guard";

const buckets: { key: "images" | "build_cache" | "containers" | "volumes"; label: string; className: string }[] = [
  { key: "images", label: "Images", className: "bg-primary" },
  { key: "build_cache", label: "Build cache", className: "bg-claude" },
  { key: "containers", label: "Containers", className: "bg-warning" },
  { key: "volumes", label: "Volumes", className: "bg-good" },
];

type Tab = "images" | "volumes" | "networks";

export function StoragePage() {
  const queryClient = useQueryClient();
  const storage = useQuery({ queryKey: ["storage"], queryFn: () => api.storage(), refetchInterval: 60_000 });
  const measure = useMutation({
    mutationFn: () => api.storage(true),
    onSuccess: (d) => {
      queryClient.setQueryData(["storage"], d);
      toast.success(`Measured: Docker uses ${formatBytes(d.df?.total_bytes ?? 0)}`, { description: `${formatBytes(d.df?.reclaimable_bytes ?? 0)} could be freed.` });
    },
    onError: (e: Error) => toast.error("Couldn't measure", { description: e.message }),
  });
  const [tab, setTab] = useState<Tab>("images");
  const s = storage.data;
  return (
    <>
      <PageHeader title="Storage" sub="What Docker keeps on disk, and what can go.">
        {s && <span className="text-xs text-muted-foreground">Measured {ago(s.measured_at)}</span>}
        <Button variant="outline" size="sm" disabled={measure.isPending} onClick={() => measure.mutate()}>
          <RefreshCw className={cn(measure.isPending && "animate-spin")} aria-hidden /> {measure.isPending ? "Measuring…" : "Measure again"}
        </Button>
      </PageHeader>
      <div className="space-y-4">
        {storage.isPending && (
          <Loading label="Measuring Docker's disk use" className="space-y-4">
            <SectionSkeleton lines={3} />
            <SectionSkeleton lines={6} />
          </Loading>
        )}
        {storage.error && <p className="text-sm text-critical">{storage.error.message}</p>}
        {s && (
          <>
            <DockerDisk s={s} />
            <Segmented
              kind="tabs"
              label="Storage views"
              value={tab}
              onChange={setTab}
              options={[
                { id: "images", label: `Images ${s.images.length}` },
                { id: "volumes", label: `Volumes ${s.volumes.length}` },
                { id: "networks", label: `Networks ${s.networks.length}` },
              ]}
            />
            {tab === "images" && <Images images={s.images} error={s.images_error} />}
            {tab === "volumes" && <Volumes volumes={s.volumes} error={s.volumes_error} />}
            {tab === "networks" && <Networks networks={s.networks} error={s.networks_error} />}
          </>
        )}
      </div>
    </>
  );
}

const prunes: { kind: PruneKind; label: string; what: string; bucket?: "images" | "build_cache" | "containers" }[] = [
  { kind: "containers", label: "Stopped containers", what: "Deletes every container that isn't running, with anything written inside it. Compose stacks recreate theirs on the next up.", bucket: "containers" },
  { kind: "dangling", label: "Untagged images", what: "Deletes images with no tag that no container uses: leftovers from rebuilds." },
  { kind: "images", label: "Unused images", what: "Deletes every image no container uses, tagged or not. They're pulled or built again when needed.", bucket: "images" },
  { kind: "build-cache", label: "Build cache", what: "Deletes all build cache. The next builds run from scratch and take longer.", bucket: "build_cache" },
  { kind: "networks", label: "Unused networks", what: "Deletes networks no container is attached to." },
];

function DockerDisk({ s }: { s: Storage }) {
  const run = useRun();
  const d = s.df;
  if (!d) {
    return (
      <Notice tone="amber" title="Docker's disk use couldn't be read">
        {s.df_error}
      </Notice>
    );
  }
  const total = Math.max(d.total_bytes, 1);
  return (
    <Section title="What Docker keeps on disk" sub={`${formatBytes(d.total_bytes)} in images, build cache, containers and volumes. About ${formatBytes(d.reclaimable_bytes)} could be freed.`}>
      <div className="flex h-2.5 gap-[2px] overflow-hidden rounded-full bg-muted" role="img" aria-label="Docker disk use by kind">
        {buckets.map((b) => d[b.key].bytes > 0 && <div key={b.key} className={b.className} style={{ width: `${(d[b.key].bytes / total) * 100}%` }} />)}
      </div>
      <dl className="mt-4 grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        {buckets.map((b) => (
          <BucketStat key={b.key} label={b.label} swatch={b.className} b={d[b.key]} />
        ))}
      </dl>
      <h3 className="mt-5 mb-2 text-[0.8125rem] text-muted-foreground">Free up space</h3>
      <div className="flex flex-wrap gap-2">
        {prunes.map((p) => {
          const size = p.bucket ? d[p.bucket].reclaimable_bytes : 0;
          return (
            <Button
              key={p.kind}
              variant="outline"
              size="sm"
              icon={Trash2}
              onClick={() =>
                void run(() => api.prune(p.kind), {
                  confirm: { title: `Prune ${p.label.toLowerCase()}?`, description: p.what, confirmLabel: "Prune" },
                  success: (r) => `Removed ${r.removed}, freed ${formatBytes(r.reclaimed)}`,
                  invalidate: [["storage"], ["containers"], ["overview"]],
                })
              }
            >
              {p.label}
              {size > 0 && <span className="text-muted-foreground">{formatBytes(size)}</span>}
            </Button>
          );
        })}
      </div>
      <p className="mt-2 text-xs text-muted-foreground">Volumes are never pruned in bulk, since they hold data. Remove unused ones one at a time below.</p>
    </Section>
  );
}

function BucketStat({ label, swatch, b }: { label: string; swatch: string; b: Bucket }) {
  return (
    <div>
      <dt className="flex items-center gap-1.5 text-sm text-muted-foreground">
        <span className={cn("size-2.5 rounded-sm", swatch)} aria-hidden /> {label}
      </dt>
      <dd className="num mt-1 text-xl font-semibold tracking-[-0.01em]">{formatBytes(b.bytes)}</dd>
      <dd className="text-xs text-muted-foreground">
        {b.count} total, {b.active} in use{b.reclaimable_bytes > 0 && `, ${formatBytes(b.reclaimable_bytes)} unused`}
      </dd>
    </div>
  );
}

const icol = columnsFor<Image>();

const imageName = (i: Image) => i.tags[0] ?? shortId(i.id);

function Images({ images, error }: { images: Image[]; error?: string }) {
  const run = useRun();
  const [selected, setSelected] = useState<RowSelectionState>({});
  const picked = images.filter((i) => selected[i.id] && i.containers.length === 0);
  const remove = (i: Image) =>
    run(() => api.removeImage(i.id, imageName(i)), {
      confirm: { title: `Remove ${imageName(i)}?`, description: `Frees up to ${formatBytes(i.size)}. It's pulled or built again if something needs it.`, confirmLabel: "Remove" },
      success: `Removed ${imageName(i)}`,
      invalidate: [["storage"]],
    });
  const removeMany = async () => {
    const r = await run(
      async () => {
        let ok = 0;
        const failed: string[] = [];
        for (const i of picked) {
          try {
            await api.removeImage(i.id, imageName(i));
            ok++;
          } catch (err) {
            if (err instanceof ApiError && err.code === "stepup_required" && ok === 0) throw err;
            failed.push(imageName(i));
          }
        }
        return { ok, failed };
      },
      {
        confirm: { title: `Remove ${picked.length} images?`, description: `Frees up to ${formatBytes(picked.reduce((n, i) => n + i.size, 0))}. Images in use are skipped.`, confirmLabel: `Remove ${picked.length}` },
        invalidate: [["storage"]],
      },
    );
    if (r) {
      setSelected({});
      if (r.failed.length) toast.warning(`Removed ${r.ok}, ${r.failed.length} failed`, { description: r.failed.join(", ") });
      else toast.success(`Removed ${r.ok} images`);
    }
  };
  const columns = [
    selectColumn(icol, imageName),
    icol.accessor((i) => i.tags[0] ?? "<none>", {
      id: "tag",
      header: "Image",
      sortFn: "text",
      cell: ({ row: { original: i } }) => (
        <>
          <span className={cn("block break-all", i.dangling ? "text-muted-foreground" : "font-medium")}>{i.tags[0] ?? "untagged"}</span>
          {i.tags.length > 1 && <span className="block text-xs break-all text-muted-foreground">also {i.tags.slice(1).join(", ")}</span>}
        </>
      ),
    }),
    icol.accessor("id", { header: "ID", enableSorting: false, cell: ({ getValue }) => <code className="literal text-xs text-muted-foreground">{shortId(getValue())}</code> }),
    icol.accessor((i) => (i.containers.length ? "in use" : "unused"), {
      id: "use",
      header: "Used by",
      filterFn: "equalsString",
      meta: { filter: { label: "Use" } },
      cell: ({ row: { original: i } }) => (i.containers.length ? <span className="text-[0.8125rem]">{i.containers.join(", ")}</span> : <Chip tone="neutral">unused</Chip>),
    }),
    icol.accessor("created", { header: "Created", sortFn: "basic", meta: { className: "whitespace-nowrap text-muted-foreground" }, cell: ({ getValue }) => ago(getValue()) }),
    icol.accessor("size", { header: "Size", sortFn: "basic", meta: { align: "right" }, cell: ({ getValue }) => formatBytes(getValue()) }),
    icol.display({
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      meta: { align: "right" },
      cell: ({ row: { original: i } }) => (
        <RowMenu
          label={`Actions for ${imageName(i)}`}
          groups={[
            [
              { label: "Copy image ID", icon: Copy, onSelect: () => copy(i.id, "the image ID") },
              ...(i.tags[0] ? [{ label: "Copy name", icon: Copy, onSelect: () => copy(i.tags[0]!, "the image name") }] : []),
            ],
            [{ label: i.containers.length ? "In use, can't remove" : "Remove", icon: Trash2, danger: true, disabled: i.containers.length > 0, onSelect: () => void remove(i) }],
          ]}
        />
      ),
    }),
  ];
  return (
    <Block title="Images" sub="Sizes include layers shared with other images. Select unused ones to remove several at once.">
      {error ? (
        <p className="text-sm text-critical">{error}</p>
      ) : (
        <DataTable
          data={images}
          columns={columns}
          getRowId={(i) => i.id}
          searchPlaceholder="Search images"
          initialSort={{ id: "size", desc: true }}
          minWidth="52rem"
          empty="No images."
          selection={{ state: selected, onChange: setSelected }}
        />
      )}
      <BulkBar count={picked.length} noun={picked.length === 1 ? "unused image" : "unused images"} onClear={() => setSelected({})}>
        <Button size="sm" variant="destructive" icon={Trash2} onClick={() => void removeMany()}>
          Remove
        </Button>
      </BulkBar>
    </Block>
  );
}

const vcol = columnsFor<Volume>();

const volumeName = (v: Volume) => (v.name.length > 40 && !v.project ? shortId(v.name) + "…" : v.name);

function Volumes({ volumes, error }: { volumes: Volume[]; error?: string }) {
  const run = useRun();
  const [, navigate] = useLocation();
  const [selected, setSelected] = useState<RowSelectionState>({});
  const picked = volumes.filter((v) => selected[v.name] && v.containers.length === 0);
  const remove = (v: Volume) =>
    run(() => api.removeVolume(v.name), {
      confirm: {
        title: `Delete the volume ${volumeName(v)}?`,
        description: `Everything stored in it${v.size > 0 ? ` (${formatBytes(v.size)})` : ""} is gone for good. There is no undo.`,
        confirmLabel: "Delete volume",
      },
      success: `Deleted ${volumeName(v)}`,
      invalidate: [["storage"]],
    });
  const removeMany = async () => {
    const r = await run(
      async () => {
        let ok = 0;
        const failed: string[] = [];
        for (const v of picked) {
          try {
            await api.removeVolume(v.name);
            ok++;
          } catch (err) {
            if (err instanceof ApiError && err.code === "stepup_required" && ok === 0) throw err;
            failed.push(volumeName(v));
          }
        }
        return { ok, failed };
      },
      {
        confirm: {
          title: `Delete ${picked.length} volumes for good?`,
          description: `Everything stored in them${picked.some((v) => v.size > 0) ? ` (${formatBytes(picked.reduce((n, v) => n + Math.max(v.size, 0), 0))})` : ""} is gone. There is no undo.`,
          confirmLabel: `Delete ${picked.length} volumes`,
        },
        invalidate: [["storage"]],
      },
    );
    if (r) {
      setSelected({});
      if (r.failed.length) toast.warning(`Deleted ${r.ok}, ${r.failed.length} failed`, { description: r.failed.join(", ") });
      else toast.success(`Deleted ${r.ok} volumes`);
    }
  };
  const columns = [
    selectColumn(vcol, volumeName),
    vcol.accessor("name", {
      header: "Volume",
      sortFn: "text",
      cell: ({ row: { original: v } }) => (
        <>
          <span className="block font-medium break-all">{volumeName(v)}</span>
          {v.project && (
            <Link href={`/stacks/${v.project}`} className="text-xs text-muted-foreground hover:text-primary">
              {v.project}
            </Link>
          )}
        </>
      ),
    }),
    vcol.accessor((v) => (v.containers.length ? "in use" : "unused"), {
      id: "use",
      header: "Used by",
      filterFn: "equalsString",
      meta: { filter: { label: "Use" } },
      cell: ({ row: { original: v } }) => (v.containers.length ? <span className="text-[0.8125rem]">{v.containers.join(", ")}</span> : <Chip tone="neutral">unused</Chip>),
    }),
    vcol.accessor("driver", { header: "Driver", meta: { className: "text-muted-foreground" } }),
    vcol.accessor("size", { header: "Size", sortFn: "basic", meta: { align: "right" }, cell: ({ getValue }) => (getValue() < 0 ? "—" : formatBytes(getValue())) }),
    vcol.display({
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      meta: { align: "right" },
      cell: ({ row: { original: v } }) => (
        <RowMenu
          label={`Actions for ${volumeName(v)}`}
          groups={[
            [
              { label: "Copy name", icon: Copy, onSelect: () => copy(v.name, "the volume name") },
              { label: "Copy folder on disk", icon: HardDrive, onSelect: () => copy(v.mountpoint, "the folder path") },
              ...(v.project ? [{ label: "Open its stack", icon: Boxes, onSelect: () => navigate(`/stacks/${v.project}`) }] : []),
            ],
            [{ label: v.containers.length ? "In use, can't delete" : "Delete", icon: Trash2, danger: true, disabled: v.containers.length > 0, onSelect: () => void remove(v) }],
          ]}
        />
      ),
    }),
  ];
  return (
    <Block title="Volumes" sub="Data that outlives containers. Only volumes no container uses can be deleted.">
      {error ? (
        <p className="text-sm text-critical">{error}</p>
      ) : (
        <DataTable
          data={volumes}
          columns={columns}
          getRowId={(v) => v.name}
          searchPlaceholder="Search volumes"
          initialSort={{ id: "size", desc: true }}
          minWidth="44rem"
          empty="No volumes."
          selection={{ state: selected, onChange: setSelected }}
        />
      )}
      <BulkBar count={picked.length} noun={picked.length === 1 ? "unused volume" : "unused volumes"} onClear={() => setSelected({})}>
        <Button size="sm" variant="destructive" icon={Trash2} onClick={() => void removeMany()}>
          Delete
        </Button>
      </BulkBar>
    </Block>
  );
}

const ncol = columnsFor<Network>();

function Networks({ networks, error }: { networks: Network[]; error?: string }) {
  const run = useRun();
  const columns = [
    ncol.accessor("name", {
      header: "Network",
      sortFn: "text",
      cell: ({ row: { original: n } }) => (
        <>
          <span className="block font-medium break-all">{n.name}</span>
          {n.project && <span className="text-xs text-muted-foreground">{n.project}</span>}
        </>
      ),
    }),
    ncol.accessor("driver", { header: "Driver", filterFn: "equalsString", meta: { filter: { label: "Driver" }, className: "text-muted-foreground" } }),
    ncol.accessor((n) => n.subnets.join(", "), { id: "subnet", header: "Subnet", enableSorting: false, meta: { className: "num text-muted-foreground" }, cell: ({ getValue }) => getValue() || "—" }),
    ncol.accessor((n) => n.containers.length, {
      id: "containers",
      header: "Containers",
      sortFn: "basic",
      cell: ({ row: { original: n } }) => (n.containers.length ? <span className="text-[0.8125rem]">{n.containers.join(", ")}</span> : <span className="text-muted-foreground">None</span>),
    }),
    ncol.display({
      id: "actions",
      header: () => <span className="sr-only">Actions</span>,
      meta: { align: "right" },
      cell: ({ row: { original: n } }) => {
        const blocked = n.builtin ? "Built in, can't remove" : n.containers.length ? "In use, can't remove" : "";
        return (
          <RowMenu
            label={`Actions for ${n.name}`}
            groups={[
              [
                { label: "Copy network ID", icon: Copy, onSelect: () => copy(n.id, "the network ID") },
                ...(n.subnets[0] ? [{ label: "Copy subnet", icon: NetworkIcon, onSelect: () => copy(n.subnets[0]!, "the subnet") }] : []),
              ],
              [
                {
                  label: blocked || "Remove",
                  icon: Trash2,
                  danger: true,
                  disabled: !!blocked,
                  onSelect: () =>
                    void run(() => api.removeNetwork(n.id, n.name), {
                      confirm: { title: `Remove the network ${n.name}?`, description: "No container uses it. Compose recreates its networks on the next up.", confirmLabel: "Remove" },
                      success: `Removed ${n.name}`,
                      invalidate: [["storage"]],
                    }),
                },
              ],
            ]}
          />
        );
      },
    }),
  ];
  return (
    <Block title="Networks" sub="Docker's built-in bridge, host and none can't be removed.">
      {error ? <p className="text-sm text-critical">{error}</p> : <DataTable data={networks} columns={columns} getRowId={(n) => n.id} searchPlaceholder="Search networks" minWidth="40rem" empty="No networks." />}
    </Block>
  );
}
