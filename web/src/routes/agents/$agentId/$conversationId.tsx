import { createClient } from "@connectrpc/connect";
import { useQuery, useTransport } from "@connectrpc/connect-query";
import { createFileRoute } from "@tanstack/react-router";
import { ArrowUp, Loader2 } from "lucide-react";
import { useEffect, useLayoutEffect, useRef, useState } from "react";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import { MessageActions } from "@/components/chat/message-actions";
import { ToolExecution } from "@/components/chat/tool-execution";
import { TypingIndicator } from "@/components/chat/typing-indicator";
import { Button } from "@/components/ui/button";
import { Textarea } from "@/components/ui/textarea";
import { ConversationService } from "@/lib/rpc/conversation/conversation_pb";
import {
	getConversation,
	getMessages,
} from "@/lib/rpc/conversation/conversation-ConversationService_connectquery";

export const Route = createFileRoute("/agents/$agentId/$conversationId")({
	component: ConversationChat,
});

interface MessageItemText {
	type: "text";
	content: string;
}

interface MessageItemToolExecution {
	type: "tool_execution";
	name: string;
	input?: string;
	result?: string;
}

type MessageItem = MessageItemText | MessageItemToolExecution;

interface Message {
	id: string;
	role: string;
	items: MessageItem[];
}

function MessageBubble({
	message,
	isStreaming,
}: {
	message: Message;
	isStreaming?: boolean;
}) {
	const isUser = message.role === "user";

	// For user messages, combine all text items into a single string
	if (isUser) {
		const textContent = message.items
			.filter((item): item is MessageItemText => item.type === "text")
			.map((item) => item.content)
			.join("\n\n");

		return (
			<div className="group flex flex-col gap-3 items-end">
				<div className="relative max-w-[80%] rounded-2xl bg-primary px-4 py-2.5 text-primary-foreground">
					<div className="prose max-w-none [&_*]:text-primary-foreground">
						<ReactMarkdown remarkPlugins={[remarkGfm]}>
							{textContent}
						</ReactMarkdown>
					</div>
				</div>
			</div>
		);
	}

	// For assistant messages, render items in order
	// Generate stable keys for items based on message id + position
	const itemKeys = message.items.map(
		(item, i) => `${message.id}-${item.type}-${i}`,
	);

	return (
		<div className="group flex flex-col gap-3 items-start">
			{message.items.map((item, index) => {
				const key = itemKeys[index];
				if (item.type === "tool_execution") {
					return (
						<ToolExecution
							key={key}
							name={item.name}
							input={item.input}
							result={item.result}
						/>
					);
				}

				// text item
				const isLastItem = index === message.items.length - 1;
				const isLastTextItem =
					isLastItem ||
					message.items.slice(index + 1).every((i) => i.type !== "text");
				return (
					<div key={key} className="relative max-w-[80%] text-foreground">
						<div className="prose max-w-none dark:prose-invert">
							<ReactMarkdown remarkPlugins={[remarkGfm]}>
								{item.content}
							</ReactMarkdown>
						</div>
						{isStreaming && isLastItem && <TypingIndicator />}
						{!isStreaming && isLastTextItem && item.content && (
							<div className="absolute -right-8 top-0">
								<MessageActions content={item.content} />
							</div>
						)}
					</div>
				);
			})}
			{isStreaming && message.items.length === 0 && <TypingIndicator />}
		</div>
	);
}

function ConversationChat() {
	const { conversationId } = Route.useParams();
	const transport = useTransport();

	const [messages, setMessages] = useState<Message[]>([]);
	const [input, setInput] = useState("");
	const [isStreaming, setIsStreaming] = useState(false);
	const [streamingItems, setStreamingItems] = useState<MessageItem[]>([]);
	const [title, setTitle] = useState<string | undefined>();
	const lastMessageRef = useRef<HTMLDivElement>(null);
	const messagesContainerRef = useRef<HTMLDivElement>(null);
	const textareaRef = useRef<HTMLTextAreaElement>(null);
	const initialLoadDone = useRef(false);
	const prevMessagesLength = useRef(0);

	const { data: conversationData } = useQuery(getConversation, {
		id: conversationId,
	});
	const { data: messagesData } = useQuery(getMessages, { conversationId });

	// Load title from conversation
	useEffect(() => {
		if (conversationData?.title) {
			setTitle(conversationData.title);
		}
	}, [conversationData]);

	// Load messages when conversation changes
	useEffect(() => {
		if (messagesData?.messages) {
			setMessages(
				messagesData.messages.map((m) => ({
					id: m.id,
					role: m.role,
					items: m.items.map((protoItem): MessageItem => {
						if (protoItem.item.case === "text") {
							return {
								type: "text",
								content: protoItem.item.value.content,
							};
						}
						if (protoItem.item.case === "toolExecution") {
							return {
								type: "tool_execution",
								name: protoItem.item.value.name,
								input: protoItem.item.value.input,
								result: protoItem.item.value.result,
							};
						}
						return { type: "text", content: "" };
					}),
				})),
			);
		}
	}, [messagesData]);

	// Scroll to bottom on initial load (useLayoutEffect to run before paint)
	useLayoutEffect(() => {
		if (messages.length > 0 && !initialLoadDone.current) {
			const container = messagesContainerRef.current;
			if (container) {
				container.scrollTop = container.scrollHeight;
			}
			initialLoadDone.current = true;
			prevMessagesLength.current = messages.length;
		}
	}, [messages]);

	// Reset state when switching conversations
	// biome-ignore lint/correctness/useExhaustiveDependencies: intentionally run on conversationId change
	useEffect(() => {
		setStreamingItems([]);
		initialLoadDone.current = false;
		prevMessagesLength.current = 0;
	}, [conversationId]);

	// Scroll new message into view (after initial load)
	useEffect(() => {
		// Only scroll if this is a new message (not initial load)
		if (
			initialLoadDone.current &&
			messages.length > prevMessagesLength.current
		) {
			lastMessageRef.current?.scrollIntoView({
				behavior: "smooth",
				block: "start",
			});
		}
		prevMessagesLength.current = messages.length;
	}, [messages]);

	// Scroll when streaming items update
	useEffect(() => {
		if (initialLoadDone.current && streamingItems.length > 0) {
			lastMessageRef.current?.scrollIntoView({
				behavior: "smooth",
				block: "start",
			});
		}
	}, [streamingItems]);

	// Scroll when streaming starts
	useEffect(() => {
		if (isStreaming) {
			lastMessageRef.current?.scrollIntoView({
				behavior: "smooth",
				block: "start",
			});
		}
	}, [isStreaming]);

	// Focus textarea on mount
	useEffect(() => {
		textareaRef.current?.focus();
	}, []);

	const sendMessage = async () => {
		if (!input.trim() || isStreaming) return;

		const userMessage = input.trim();
		const pendingUserMessageId = "pending-user";
		setInput("");
		if (textareaRef.current) {
			textareaRef.current.style.height = "auto";
			textareaRef.current.focus();
		}
		setMessages((prev) => [
			...prev,
			{
				id: pendingUserMessageId,
				role: "user",
				items: [{ type: "text", content: userMessage }],
			},
		]);
		setIsStreaming(true);
		setStreamingItems([]);

		try {
			const client = createClient(ConversationService, transport);
			const stream = client.chat({ conversationId, content: userMessage });

			const items: MessageItem[] = [];

			for await (const event of stream) {
				if (event.event.case === "delta") {
					// Append to last text item, or create a new one
					const lastItem = items[items.length - 1];
					if (lastItem && lastItem.type === "text") {
						lastItem.content += event.event.value.content;
					} else {
						items.push({
							type: "text",
							content: event.event.value.content,
						});
					}
					setStreamingItems([...items]);
				} else if (event.event.case === "toolExecution") {
					const { name, input, result } = event.event.value;
					items.push({
						type: "tool_execution",
						name,
						input,
						result,
					});
					setStreamingItems([...items]);
				} else if (event.event.case === "done") {
					const {
						userMessageId,
						assistantMessageId,
						title: newTitle,
					} = event.event.value;
					if (newTitle) {
						setTitle(newTitle);
					}
					setMessages((prev) =>
						prev
							.map((m) =>
								m.id === pendingUserMessageId ? { ...m, id: userMessageId } : m,
							)
							.concat({
								id: assistantMessageId,
								role: "assistant",
								items: [...items],
							}),
					);
					setStreamingItems([]);
				} else if (event.event.case === "error") {
					console.error("Chat error:", event.event.value.message);
				}
			}
		} catch (err) {
			console.error("Stream error:", err);
		} finally {
			setIsStreaming(false);
		}
	};

	const handleKeyDown = (e: React.KeyboardEvent) => {
		if (e.key === "Enter" && !e.shiftKey) {
			e.preventDefault();
			sendMessage();
		}
	};

	return (
		<div className="relative flex h-[calc(100vh-8rem)] flex-col">
			{/* Header */}
			<div className="flex items-center justify-between border-b pb-4">
				<h1 className="text-lg font-semibold">{title || "Chat"}</h1>
			</div>

			{/* Messages */}
			<div
				ref={messagesContainerRef}
				className="flex-1 overflow-y-auto pb-24 pt-4"
			>
				<div className="mx-auto max-w-3xl">
					{messages.length === 0 && streamingItems.length === 0 ? (
						<div className="flex h-full items-center justify-center">
							<p className="text-muted-foreground">
								Send a message to start the conversation
							</p>
						</div>
					) : (
						<div className="space-y-4">
							{messages.map((msg, index) => {
								const isLast = index === messages.length - 1;
								const shouldRef = isLast && streamingItems.length === 0;
								return (
									<div key={msg.id} ref={shouldRef ? lastMessageRef : null}>
										<MessageBubble message={msg} />
									</div>
								);
							})}
							{streamingItems.length > 0 && (
								<div ref={lastMessageRef}>
									<MessageBubble
										message={{
											id: "streaming",
											role: "assistant",
											items: streamingItems,
										}}
										isStreaming
									/>
								</div>
							)}
						</div>
					)}
				</div>
			</div>

			{/* Input */}
			<div className="absolute inset-x-0 bottom-0 bg-gradient-to-t from-background via-background to-transparent pb-4 pt-6">
				<div className="mx-auto max-w-3xl">
					<div className="flex items-end gap-2 rounded-lg border bg-background p-2 shadow-sm">
						<Textarea
							ref={textareaRef}
							value={input}
							onChange={(e) => {
								setInput(e.target.value);
								e.target.style.height = "auto";
								e.target.style.height = `${e.target.scrollHeight}px`;
							}}
							onKeyDown={handleKeyDown}
							placeholder="Type a message..."
							className="min-h-0 max-h-48 flex-1 resize-none border-0 bg-transparent p-2 shadow-none focus-visible:ring-0"
							rows={1}
						/>
						<Button
							onClick={sendMessage}
							disabled={isStreaming || !input.trim()}
							size="icon"
							className="h-9 w-9 shrink-0"
						>
							{isStreaming ? (
								<Loader2 className="h-4 w-4 animate-spin" />
							) : (
								<ArrowUp className="h-4 w-4" />
							)}
							<span className="sr-only">Send message</span>
						</Button>
					</div>
					<p className="mt-2 text-center text-xs text-muted-foreground">
						Press Enter to send, Shift+Enter for new line
					</p>
				</div>
			</div>
		</div>
	);
}
