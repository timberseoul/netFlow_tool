import { useEffect, useMemo, useState } from "react";
import { flexRender, getCoreRowModel, useReactTable } from "@tanstack/react-table";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { Badge } from "./components/ui/badge";
import { Card, CardContent, CardHeader, CardTitle } from "./components/ui/card";
import { Tabs, TabsButton } from "./components/ui/tabs";

const categoryLabels = {
  "": "All",
  user: "User",
  system: "System",
  service: "Service",
  unknown: "Unknown",
};

const reconnectBaseDelayMs = 2000;
const reconnectMaxDelayMs = 10000;

function formatBytes(bytes) {
  if (bytes >= 1024 ** 3) return `${(bytes / 1024 ** 3).toFixed(2)} GB`;
  if (bytes >= 1024 ** 2) return `${(bytes / 1024 ** 2).toFixed(2)} MB`;
  if (bytes >= 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${bytes} B`;
}

function formatSpeed(bytesPerSec) {
  if (bytesPerSec >= 1024 ** 3) return `${(bytesPerSec / 1024 ** 3).toFixed(2)} GB/s`;
  if (bytesPerSec >= 1024 ** 2) return `${(bytesPerSec / 1024 ** 2).toFixed(2)} MB/s`;
  if (bytesPerSec >= 1024) return `${(bytesPerSec / 1024).toFixed(2)} KB/s`;
  return `${bytesPerSec.toFixed(0)} B/s`;
}

function comparePrimitive(left, right) {
  if (typeof left === "string" || typeof right === "string") {
    return String(left ?? "").localeCompare(String(right ?? ""));
  }
  return Number(left ?? 0) - Number(right ?? 0);
}

function buildTreeRows(flows, expandedParents, sorting) {
  const nodes = new Map();
  const orderedNodes = flows.map((flow) => {
    const node = { flow, children: [] };
    nodes.set(flow.pid, node);
    return node;
  });

  const roots = [];
  for (const node of orderedNodes) {
    const parentPid = node.flow.parent_pid;
    if (parentPid && nodes.has(parentPid) && parentPid !== node.flow.pid) {
      nodes.get(parentPid).children.push(node);
      continue;
    }
    roots.push(node);
  }

  function aggregateNode(node) {
    let uploadSpeed = node.flow.upload_speed;
    let downloadSpeed = node.flow.download_speed;
    let totalUpload = node.flow.total_upload;
    let totalDownload = node.flow.total_download;

    for (const child of node.children) {
      const childAggregate = aggregateNode(child);
      uploadSpeed += childAggregate.uploadSpeed;
      downloadSpeed += childAggregate.downloadSpeed;
      totalUpload += childAggregate.totalUpload;
      totalDownload += childAggregate.totalDownload;
    }

    return { uploadSpeed, downloadSpeed, totalUpload, totalDownload };
  }

  function compareNodes(left, right) {
    const [activeSort] = sorting;
    const sortId = activeSort?.id ?? "aggregate_download_speed";
    const desc = activeSort?.desc ?? true;
    const leftAggregate = aggregateNode(left);
    const rightAggregate = aggregateNode(right);
    let comparison = 0;

    switch (sortId) {
      case "name":
        comparison = comparePrimitive(left.flow.name, right.flow.name);
        break;
      case "upload_speed":
      case "aggregate_upload_speed":
        comparison = comparePrimitive(leftAggregate.uploadSpeed, rightAggregate.uploadSpeed);
        break;
      case "download_speed":
      case "aggregate_download_speed":
        comparison = comparePrimitive(leftAggregate.downloadSpeed, rightAggregate.downloadSpeed);
        break;
      case "total_upload":
      case "aggregate_total_upload":
        comparison = comparePrimitive(leftAggregate.totalUpload, rightAggregate.totalUpload);
        break;
      case "total_download":
      case "aggregate_total_download":
        comparison = comparePrimitive(leftAggregate.totalDownload, rightAggregate.totalDownload);
        break;
      case "pid":
        comparison = comparePrimitive(left.flow.pid, right.flow.pid);
        break;
      default:
        comparison = comparePrimitive(leftAggregate.downloadSpeed, rightAggregate.downloadSpeed);
        break;
    }

    if (comparison === 0) {
      comparison = comparePrimitive(left.flow.pid, right.flow.pid);
    }

    return desc ? -comparison : comparison;
  }

  function sortNodes(nodesToSort) {
    nodesToSort.sort(compareNodes);
    for (const node of nodesToSort) {
      if (node.children.length > 0) {
        sortNodes(node.children);
      }
    }
  }

  sortNodes(roots);

  const rows = [];
  function appendNode(node, depth) {
    const hasChildren = node.children.length > 0;
    const expanded = hasChildren && Boolean(expandedParents[node.flow.pid]);
    const aggregate = aggregateNode(node);
    rows.push({
      ...node.flow,
      depth,
      hasChildren,
      expanded,
      aggregate_upload_speed: aggregate.uploadSpeed,
      aggregate_download_speed: aggregate.downloadSpeed,
      aggregate_total_upload: aggregate.totalUpload,
      aggregate_total_download: aggregate.totalDownload,
    });

    if (!expanded) return;
    for (const child of node.children) {
      appendNode(child, depth + 1);
    }
  }

  for (const root of roots) {
    appendNode(root, 0);
  }

  return rows;
}

function StatCard({ label, value, accent }) {
  return (
    <Card>
      <CardContent>
        <div className="text-xs uppercase tracking-wide text-zinc-400">{label}</div>
        <div className={`mt-3 text-2xl font-bold ${accent ?? "text-zinc-50"}`}>{value}</div>
      </CardContent>
    </Card>
  );
}

export default function App() {
  const [flows, setFlows] = useState([]);
  const [history, setHistory] = useState([]);
  const [status, setStatus] = useState("connecting");
  const [filterCategory, setFilterCategory] = useState("");
  const [sorting, setSorting] = useState([{ id: "aggregate_download_speed", desc: true }]);
  const [chartPoints, setChartPoints] = useState([]);
  const [expandedParents, setExpandedParents] = useState({});

  function toggleColumnSort(field) {
    setSorting((current) => {
      if ((current[0]?.id ?? "aggregate_download_speed") === field) {
        return [{ id: field, desc: !(current[0]?.desc ?? true) }];
      }
      return [{ id: field, desc: true }];
    });
  }

  function sortIndicator(field) {
    if (sorting[0]?.id !== field) return "↕";
    return sorting[0]?.desc ? "↓" : "↑";
  }

  function renderSortableHeader(label, field) {
    const active = sorting[0]?.id === field;
    return (
      <button
        type="button"
        className={`inline-flex items-center gap-1 rounded-md px-2 py-1 transition ${
          active ? "bg-zinc-800 text-zinc-50" : "text-zinc-400 hover:bg-zinc-900 hover:text-zinc-100"
        }`}
        onClick={() => toggleColumnSort(field)}
      >
        <span>{label}</span>
        <span className={active ? "text-sky-300" : "text-zinc-500"}>{sortIndicator(field)}</span>
      </button>
    );
  }

  useEffect(() => {
    let mounted = true;
    let ws = null;
    let reconnectTimer = null;
    let reconnectAttempts = 0;

    function applySnapshot(payload) {
      if (!mounted) return;
      setFlows(payload.flows || []);
      if (Array.isArray(payload.history)) {
        setHistory(payload.history);
      }
      setChartPoints(
        (payload.throughput || []).map((point) => ({
          time: point.label || new Date(point.timestamp).toLocaleTimeString(),
          upload: point.upload_speed,
          download: point.download_speed,
          sampleCount: point.sample_count,
        })),
      );
    }

    function fetchBootstrap() {
      return fetch("/api/bootstrap")
        .then((response) => response.json())
        .then((payload) => {
          applySnapshot(payload);
        })
        .catch(() => {
          if (mounted) setStatus("bootstrap-error");
        });
    }

    function scheduleReconnect() {
      if (!mounted || reconnectTimer !== null) return;

      const delay = Math.min(
        reconnectBaseDelayMs * 2 ** Math.min(reconnectAttempts, 2),
        reconnectMaxDelayMs,
      );
      reconnectAttempts += 1;
      setStatus("reconnecting");
      reconnectTimer = window.setTimeout(() => {
        reconnectTimer = null;
        connectWebSocket();
      }, delay);
    }

    function connectWebSocket() {
      if (!mounted) return;

      const protocol = window.location.protocol === "https:" ? "wss" : "ws";
      ws = new WebSocket(`${protocol}://${window.location.host}/ws`);

      ws.onopen = () => {
        reconnectAttempts = 0;
        setStatus("connected");
        void fetchBootstrap();
      };

      ws.onclose = () => {
        if (!mounted) return;
        setStatus("disconnected");
        scheduleReconnect();
      };

      ws.onerror = () => {
        if (mounted) setStatus("error");
      };

      ws.onmessage = (event) => {
        const payload = JSON.parse(event.data);
        if (payload.type !== "snapshot") return;
        applySnapshot(payload);
      };
    }

    void fetchBootstrap();
    connectWebSocket();

    return () => {
      mounted = false;
      if (reconnectTimer !== null) {
        window.clearTimeout(reconnectTimer);
      }
      if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
        ws.close();
      }
    };
  }, []);

  const visibleFlows = useMemo(
    () => flows.filter((flow) => !filterCategory || flow.category === filterCategory),
    [flows, filterCategory],
  );

  const treeRows = useMemo(
    () => buildTreeRows(visibleFlows, expandedParents, sorting),
    [expandedParents, sorting, visibleFlows],
  );

  const totals = useMemo(
    () =>
      visibleFlows.reduce(
        (acc, flow) => {
          acc.uploadSpeed += flow.upload_speed;
          acc.downloadSpeed += flow.download_speed;
          acc.totalUpload += flow.total_upload;
          acc.totalDownload += flow.total_download;
          return acc;
        },
        { uploadSpeed: 0, downloadSpeed: 0, totalUpload: 0, totalDownload: 0 },
      ),
    [visibleFlows],
  );

  const categorySeries = useMemo(() => {
    const buckets = {};
    for (const flow of flows) {
      const key = flow.category || "unknown";
      if (!buckets[key]) {
        buckets[key] = { category: categoryLabels[key] || key, upload: 0, download: 0 };
      }
      buckets[key].upload += flow.upload_speed;
      buckets[key].download += flow.download_speed;
    }
    return Object.values(buckets);
  }, [flows]);

  const historyRows = useMemo(
    () =>
      [...history]
        .sort((a, b) => b.date.localeCompare(a.date))
        .slice(0, 7)
        .map((row) => ({
          ...row,
          total: row.upload + row.download,
        })),
    [history],
  );

  const columns = useMemo(
    () => [
      { accessorKey: "pid", header: "PID" },
      {
        accessorKey: "name",
        header: () => renderSortableHeader("Process", "name"),
        cell: ({ row }) => {
          const item = row.original;
          const indent = { paddingLeft: `${item.depth * 1.25}rem` };
          const arrow = item.hasChildren ? (item.expanded ? "▼" : "▶") : item.depth > 0 ? "└─" : "•";
          const isChild = item.depth > 0;
          const textClass = isChild ? "text-sky-200" : "text-zinc-100";

          return (
            <div className={`flex items-center gap-2 ${textClass}`} style={indent}>
              {item.hasChildren ? (
                <button
                  type="button"
                  className="w-5 text-left text-zinc-400 transition hover:text-zinc-100"
                  onClick={() =>
                    setExpandedParents((current) => ({
                      ...current,
                      [item.pid]: !current[item.pid],
                    }))
                  }
                >
                  {arrow}
                </button>
              ) : (
                <span className="w-5 text-sky-400/70">{arrow}</span>
              )}
              <span className={item.hasChildren ? "font-semibold" : "font-medium"}>{item.name}</span>
            </div>
          );
        },
      },
      {
        accessorKey: "category",
        header: "Category",
        cell: ({ getValue }) => categoryLabels[getValue()] || getValue(),
      },
      {
        accessorKey: "status",
        header: "Status",
        cell: ({ getValue }) => (
          <Badge className={getValue() === "active" ? "bg-emerald-900/60 text-emerald-300" : "bg-zinc-800 text-zinc-400"}>
            {getValue()}
          </Badge>
        ),
      },
      {
        accessorKey: "aggregate_upload_speed",
        header: () => renderSortableHeader("Upload", "aggregate_upload_speed"),
        cell: ({ getValue }) => formatSpeed(getValue()),
      },
      {
        accessorKey: "aggregate_download_speed",
        header: () => renderSortableHeader("Download", "aggregate_download_speed"),
        cell: ({ getValue }) => formatSpeed(getValue()),
      },
      {
        accessorKey: "aggregate_total_upload",
        header: "Total Up",
        cell: ({ getValue }) => formatBytes(getValue()),
      },
      {
        accessorKey: "aggregate_total_download",
        header: "Total Down",
        cell: ({ getValue }) => formatBytes(getValue()),
      },
    ],
    [sorting],
  );

  const table = useReactTable({
    data: treeRows,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
  });

  return (
    <div className="min-h-screen bg-zinc-950 px-6 py-6 text-zinc-50">
      <div className="mx-auto flex max-w-7xl flex-col gap-6">
        <Card className="shadow-xl">
          <CardContent className="flex flex-col gap-4">
            <div className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
              <div>
                <h1 className="text-3xl font-bold tracking-tight text-zinc-50">netFlow_tool WebUI</h1>
                <p className="mt-1 text-sm text-zinc-400">
                  基于 Go 本地 HTTP + WebSocket 的前后端分离监测面板。
                </p>
              </div>
              <div className="flex flex-wrap items-center gap-3">
                <Badge className={status === "connected" ? "bg-emerald-900/60 text-emerald-300" : "bg-amber-900/60 text-amber-300"}>
                  WebSocket: {status}
                </Badge>
                <Tabs>
                  {["", "user", "system", "service"].map((filter) => (
                    <TabsButton
                      key={filter || "all"}
                      active={filterCategory === filter}
                      onClick={() => setFilterCategory(filter)}
                    >
                      {categoryLabels[filter]}
                    </TabsButton>
                  ))}
                </Tabs>
              </div>
            </div>
          </CardContent>
        </Card>

        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          <StatCard label="Visible Upload" value={formatSpeed(totals.uploadSpeed)} accent="text-rose-300" />
          <StatCard label="Visible Download" value={formatSpeed(totals.downloadSpeed)} accent="text-emerald-300" />
          <StatCard label="Total Upload" value={formatBytes(totals.totalUpload)} />
          <StatCard label="Total Download" value={formatBytes(totals.totalDownload)} />
        </div>

        <div className="grid gap-6 xl:grid-cols-[1.5fr_1fr]">
          <Card>
            <CardHeader>
              <CardTitle>2-Min Cached Average Throughput</CardTitle>
            </CardHeader>
            <CardContent className="h-80">
              <ResponsiveContainer width="100%" height="100%">
                <LineChart data={chartPoints}>
                  <CartesianGrid stroke="#27272a" strokeDasharray="3 3" />
                  <XAxis dataKey="time" stroke="#a1a1aa" minTickGap={24} />
                  <YAxis stroke="#a1a1aa" tickFormatter={(value) => formatSpeed(value)} />
                  <Tooltip
                    contentStyle={{ backgroundColor: "#111114", border: "1px solid #27272a", borderRadius: "12px" }}
                    formatter={(value, name, item) => [
                      formatSpeed(value),
                      item?.payload?.sampleCount ? `${name} (${item.payload.sampleCount} samples avg)` : name,
                    ]}
                  />
                  <Legend />
                  <Line type="monotone" dataKey="upload" stroke="#fda4af" strokeWidth={2} dot={false} name="Upload" />
                  <Line type="monotone" dataKey="download" stroke="#86efac" strokeWidth={2} dot={false} name="Download" />
                </LineChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Category Distribution</CardTitle>
            </CardHeader>
            <CardContent className="h-80">
              <ResponsiveContainer width="100%" height="100%">
                <BarChart data={categorySeries}>
                  <CartesianGrid stroke="#27272a" strokeDasharray="3 3" />
                  <XAxis dataKey="category" stroke="#a1a1aa" />
                  <YAxis stroke="#a1a1aa" tickFormatter={(value) => formatSpeed(value)} />
                  <Tooltip
                    contentStyle={{ backgroundColor: "#111114", border: "1px solid #27272a", borderRadius: "12px" }}
                    formatter={(value) => formatSpeed(value)}
                  />
                  <Legend />
                  <Bar dataKey="upload" fill="#fda4af" radius={[8, 8, 0, 0]} name="Upload" />
                  <Bar dataKey="download" fill="#86efac" radius={[8, 8, 0, 0]} name="Download" />
                </BarChart>
              </ResponsiveContainer>
            </CardContent>
          </Card>
        </div>

        <div className="grid gap-6 xl:grid-cols-[1.5fr_1fr]">
          <Card>
            <CardHeader>
              <CardTitle>Process Flows Tree</CardTitle>
              <p className="text-sm text-zinc-400">基于最近 2 分钟缓存样本平均值渲染，与吞吐量曲线保持一致。</p>
            </CardHeader>
            <CardContent className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="border-b border-zinc-800 text-zinc-400">
                  {table.getHeaderGroups().map((headerGroup) => (
                    <tr key={headerGroup.id}>
                      {headerGroup.headers.map((header) => (
                        <th key={header.id} className="px-3 py-3 text-left font-medium">
                          {flexRender(header.column.columnDef.header, header.getContext())}
                        </th>
                      ))}
                    </tr>
                  ))}
                </thead>
                <tbody>
                  {table.getRowModel().rows.map((row) => (
                    <tr
                      key={row.id}
                      className={`border-b border-zinc-800/80 text-zinc-200 hover:bg-zinc-900/50 ${
                        row.original.hasChildren
                          ? "bg-zinc-900/50 shadow-inner"
                          : row.original.depth > 0
                            ? "bg-sky-950/25"
                            : ""
                      }`}
                    >
                      {row.getVisibleCells().map((cell) => (
                        <td key={cell.id} className="px-3 py-3">
                          {flexRender(cell.column.columnDef.cell, cell.getContext())}
                        </td>
                      ))}
                    </tr>
                  ))}
                </tbody>
              </table>
            </CardContent>
          </Card>

          <Card>
            <CardHeader>
              <CardTitle>Recent Daily Usage</CardTitle>
            </CardHeader>
            <CardContent className="space-y-3">
              {historyRows.map((row) => (
                <div key={row.date} className="rounded-lg border border-zinc-800 bg-zinc-950/60 p-3">
                  <div className="text-sm font-medium text-zinc-100">{row.date.slice(0, 10)}</div>
                  <div className="mt-2 text-xs text-zinc-400">
                    Upload: {formatBytes(row.upload)} | Download: {formatBytes(row.download)}
                  </div>
                  <div className="mt-1 text-xs text-zinc-500">Total: {formatBytes(row.total)}</div>
                </div>
              ))}
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
