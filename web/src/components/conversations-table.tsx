import { Link } from "@tanstack/react-router";
import {
	type ColumnDef,
	flexRender,
	getCoreRowModel,
	getSortedRowModel,
	type SortingState,
	useReactTable,
} from "@tanstack/react-table";
import { ArrowUpDown, MessageSquare } from "lucide-react";
import { useState } from "react";
import { Button } from "@/components/ui/button";
import {
	Table,
	TableBody,
	TableCell,
	TableHead,
	TableHeader,
	TableRow,
} from "@/components/ui/table";
import type { Conversation } from "@/lib/rpc/conversation/conversation_pb";

interface ConversationsTableProps {
	conversations: Conversation[];
	agentId: string;
}

export function ConversationsTable({
	conversations,
	agentId,
}: ConversationsTableProps) {
	const [sorting, setSorting] = useState<SortingState>([]);

	const columns: ColumnDef<Conversation>[] = [
		{
			accessorKey: "title",
			header: ({ column }) => (
				<Button
					variant="ghost"
					onClick={() => column.toggleSorting(column.getIsSorted() === "asc")}
				>
					Title
					<ArrowUpDown className="ml-2 h-4 w-4" />
				</Button>
			),
			cell: ({ row }) => {
				const conv = row.original;
				return (
					<Link
						to="/agents/$agentId/$conversationId"
						params={{ agentId, conversationId: conv.id }}
						className="flex max-w-md items-center gap-2 font-medium hover:underline"
					>
						<MessageSquare className="h-4 w-4 shrink-0 text-muted-foreground" />
						<span className="truncate">
							{conv.title || `Conversation ${conv.id.slice(0, 8)}`}
						</span>
					</Link>
				);
			},
		},
		{
			accessorKey: "id",
			header: "ID",
			cell: ({ row }) => (
				<span className="font-mono text-sm text-muted-foreground">
					{row.original.id.slice(0, 8)}...
				</span>
			),
		},
	];

	const table = useReactTable({
		data: conversations,
		columns,
		getCoreRowModel: getCoreRowModel(),
		getSortedRowModel: getSortedRowModel(),
		onSortingChange: setSorting,
		state: { sorting },
	});

	return (
		<div className="rounded-md border">
			<Table>
				<TableHeader>
					{table.getHeaderGroups().map((headerGroup) => (
						<TableRow key={headerGroup.id}>
							{headerGroup.headers.map((header) => (
								<TableHead key={header.id}>
									{header.isPlaceholder
										? null
										: flexRender(
												header.column.columnDef.header,
												header.getContext(),
											)}
								</TableHead>
							))}
						</TableRow>
					))}
				</TableHeader>
				<TableBody>
					{table.getRowModel().rows?.length ? (
						table.getRowModel().rows.map((row) => (
							<TableRow key={row.id}>
								{row.getVisibleCells().map((cell) => (
									<TableCell key={cell.id}>
										{flexRender(cell.column.columnDef.cell, cell.getContext())}
									</TableCell>
								))}
							</TableRow>
						))
					) : (
						<TableRow>
							<TableCell colSpan={columns.length} className="h-24 text-center">
								No conversations yet
							</TableCell>
						</TableRow>
					)}
				</TableBody>
			</Table>
		</div>
	);
}
