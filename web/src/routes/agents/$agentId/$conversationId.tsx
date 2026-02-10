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
  isBusy,
}: {
  message: Message;
  isBusy?: boolean;
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
          <div className="prose max-w-none **:text-primary-foreground">
            <ReactMarkdown remarkPlugins={[remarkGfm]}>
              {textContent}
            </ReactMarkdown>
          </div>
        </div>
      </div>
    );
  }

  // For assistant messages, render items in order
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
            {isBusy && isLastItem && <TypingIndicator />}
            {!isBusy && isLastTextItem && item.content && (
              <div className="absolute -right-8 top-0">
                <MessageActions content={item.content} />
              </div>
            )}
          </div>
        );
      })}
      {isBusy && message.items.length === 0 && <TypingIndicator />}
    </div>
  );
}

function ConversationChat() {
  const { conversationId } = Route.useParams();
  const transport = useTransport();

  const [messages, setMessages] = useState<Message[]>([]);
  const [input, setInput] = useState("");
  const [isBusy, setIsBusy] = useState(false);
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
    setIsBusy(false);
    initialLoadDone.current = false;
    prevMessagesLength.current = 0;
  }, [conversationId]);

  // Scroll new message into view (after initial load)
  useEffect(() => {
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

  // Scroll when busy state changes to true
  useEffect(() => {
    if (isBusy) {
      lastMessageRef.current?.scrollIntoView({
        behavior: "smooth",
        block: "start",
      });
    }
  }, [isBusy]);

  // WatchEvents subscription
  useEffect(() => {
    const controller = new AbortController();
    const client = createClient(ConversationService, transport);

    (async () => {
      try {
        const stream = client.watchEvents(
          { conversationId },
          { signal: controller.signal },
        );

        const items: MessageItem[] = [];

        for await (const event of stream) {
          switch (event.event.case) {
            case "turnStarted":
              setIsBusy(true);
              break;

            case "textDelta": {
              setIsBusy(true);
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
              break;
            }

            case "toolResult":
              setIsBusy(true);
              items.push({
                type: "tool_execution",
                name: event.event.value.name,
                input: event.event.value.input,
                result: event.event.value.result,
              });
              setStreamingItems([...items]);
              break;

            case "messageCreated": {
              const msg = event.event.value.message;
              if (!msg) break;

              const messageItems: MessageItem[] = msg.items.map(
                (protoItem): MessageItem => {
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
                },
              );

              const newMessage: Message = {
                id: msg.id,
                role: msg.role,
                items: messageItems,
              };

              if (msg.role === "assistant") {
                // Clear streaming items and add final message
                items.length = 0;
                setStreamingItems([]);
              }

              setMessages((prev) => {
                // Replace optimistic message or skip if already present
                const existingIndex = prev.findIndex(
                  (m) => m.id === msg.id || m.id === "pending-user",
                );
                if (
                  existingIndex >= 0 &&
                  (prev[existingIndex].id === "pending-user" ||
                    prev[existingIndex].id === msg.id)
                ) {
                  const updated = [...prev];
                  updated[existingIndex] = newMessage;
                  return updated;
                }
                return [...prev, newMessage];
              });
              break;
            }

            case "done":
              setIsBusy(false);
              if (event.event.value.title) {
                setTitle(event.event.value.title);
              }
              // Reset streaming state for next turn
              items.length = 0;
              setStreamingItems([]);
              break;

            case "error":
              setIsBusy(false);
              console.error("Watch error:", event.event.value.message);
              items.length = 0;
              setStreamingItems([]);
              break;
          }
        }
      } catch (err) {
        // AbortError is expected on cleanup
        if (err instanceof DOMException && err.name === "AbortError") {
          return;
        }
        console.error("WatchEvents stream error:", err);
      }
    })();

    return () => controller.abort();
  }, [conversationId, transport]);

  // Dismiss keyboard when scrolling to the top
  useEffect(() => {
    const container = messagesContainerRef.current;
    if (!container) return;

    let lastScrollTop = container.scrollTop;

    const handleScroll = () => {
      const scrollTop = container.scrollTop;
      const isScrollingUp = scrollTop < lastScrollTop;
      lastScrollTop = scrollTop;

      if (isScrollingUp && scrollTop <= 0) {
        (document.activeElement as HTMLElement)?.blur();
      }
    };

    container.addEventListener("scroll", handleScroll, { passive: true });
    return () => container.removeEventListener("scroll", handleScroll);
  }, []);

  const sendMessage = async () => {
    if (!input.trim() || isBusy) return;

    const userMessage = input.trim();
    setInput("");
    if (textareaRef.current) {
      textareaRef.current.style.height = "auto";
    }

    // Optimistic user message
    setMessages((prev) => [
      ...prev,
      {
        id: "pending-user",
        role: "user",
        items: [{ type: "text", content: userMessage }],
      },
    ]);
    setIsBusy(true);

    try {
      const client = createClient(ConversationService, transport);
      const resp = await client.chat({
        conversationId,
        content: userMessage,
      });

      // Patch optimistic message with real ID
      setMessages((prev) =>
        prev.map((m) =>
          m.id === "pending-user" ? { ...m, id: resp.userMessageId } : m,
        ),
      );
    } catch (err) {
      console.error("Chat error:", err);
      // Remove optimistic message on failure
      setMessages((prev) => prev.filter((m) => m.id !== "pending-user"));
      setIsBusy(false);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      sendMessage();
    }
  };

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      {/* Header */}
      <div className="flex items-center justify-between border-b px-4 pb-4 pt-4 md:px-6">
        <h1 className="text-lg font-semibold">{title || "Chat"}</h1>
      </div>

      {/* Messages */}
      <div
        ref={messagesContainerRef}
        className="flex-1 overflow-y-auto overscroll-contain pt-4"
      >
        <div className="mx-auto max-w-3xl px-4 md:px-6">
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
                    isBusy
                  />
                </div>
              )}
              {isBusy && streamingItems.length === 0 && messages.length > 0 && (
                <div ref={lastMessageRef}>
                  <MessageBubble
                    message={{
                      id: "waiting",
                      role: "assistant",
                      items: [],
                    }}
                    isBusy
                  />
                </div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Input */}
      <div className="shrink-0 pt-2 pb-[env(safe-area-inset-bottom)] md:pb-4 md:px-6">
        <div className="mx-auto max-w-3xl px-4 md:px-6">
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
              disabled={isBusy || !input.trim()}
              size="icon"
              className="h-9 w-9 shrink-0"
            >
              {isBusy ? (
                <Loader2 className="h-4 w-4 animate-spin" />
              ) : (
                <ArrowUp className="h-4 w-4" />
              )}
              <span className="sr-only">Send message</span>
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
}
