import { useEffect, useMemo, useState } from "react";
import {
  flexRender,
  getCoreRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table";
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
  const [sorting, setSorting] = useState([{ id: "download_speed", desc: true }]);
  const [chartPoints, setChartPoints] = useState([]);

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
        header: "Process",
        cell: ({ getValue }) => <span className="font-medium text-zinc-100">{getValue()}</span>,
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
        accessorKey: "upload_speed",
        header: "Upload",
        cell: ({ getValue }) => formatSpeed(getValue()),
      },
      {
        accessorKey: "download_speed",
        header: "Download",
        cell: ({ getValue }) => formatSpeed(getValue()),
      },
      {
        accessorKey: "total_upload",
        header: "Total Up",
        cell: ({ getValue }) => formatBytes(getValue()),
      },
      {
        accessorKey: "total_download",
        header: "Total Down",
        cell: ({ getValue }) => formatBytes(getValue()),
      },
    ],
    [],
  );

  const table = useReactTable({
    data: visibleFlows,
    columns,
    state: { sorting },
    onSortingChange: setSorting,
    getCoreRowModel: getCoreRowModel(),
    getSortedRowModel: getSortedRowModel(),
  });

  return (
    <div className="min-h-screen bg-zinc-950 px-6 py-6 text-zinc-50">
      <div className="mx-auto flex max-w-7xl flex-col gap-6">
        <Card className="shadow-xl">
          <CardContent className="flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
            <div>
              <h1 className="text-3xl font-bold tracking-tight text-zinc-50">netFlow_tool WebUI</h1>
              <p className="mt-1 text-sm text-zinc-400">
                基于 Go 本地 HTTP + WebSocket 的前后端分离监测面板。
              </p>
            </div>
            <div className="flex items-center gap-3">
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
              <CardTitle>Process Flows</CardTitle>
            </CardHeader>
            <CardContent className="overflow-x-auto">
              <table className="min-w-full text-sm">
                <thead className="border-b border-zinc-800 text-zinc-400">
                  {table.getHeaderGroups().map((headerGroup) => (
                    <tr key={headerGroup.id}>
                      {headerGroup.headers.map((header) => (
                        <th
                          key={header.id}
                          className="cursor-pointer px-3 py-3 text-left font-medium"
                          onClick={header.column.getToggleSortingHandler()}
                        >
                          {flexRender(header.column.columnDef.header, header.getContext())}
                        </th>
                      ))}
                    </tr>
                  ))}
                </thead>
                <tbody>
                  {table.getRowModel().rows.map((row) => (
                    <tr key={row.id} className="border-b border-zinc-800/80 text-zinc-200 hover:bg-zinc-900/50">
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
