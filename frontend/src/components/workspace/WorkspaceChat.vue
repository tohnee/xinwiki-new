<script setup lang="ts">
import { ref, nextTick, onMounted, onUnmounted, computed, watch } from 'vue'
import { useStream } from '@/api/chat/streame'
import { createSessions } from '@/api/chat'
import { SendIcon, StopIcon } from 'tdesign-icons-vue-next'
import { sanitizeMarkdownHTML } from '@/utils/security'
import { MessagePlugin } from 'tdesign-vue-next'

export interface ChatMessage {
  id: string
  role: 'user' | 'assistant'
  content: string
  thinking?: string
  thinkingActive?: boolean
  references?: Array<{ id: string; title: string; excerpt?: string; url?: string }>
  isStreaming?: boolean
  isCompleted?: boolean
  error?: string
  createdAt: number
}

const props = defineProps<{
  initialSessionId?: string
  knowledgeBaseIds?: string[]
}>()

const sessionId = ref<string>(props.initialSessionId || '')
const messages = ref<ChatMessage[]>([])
const inputText = ref('')
const isSending = ref(false)
const scrollContainer = ref<HTMLElement | null>(null)
const textareaRef = ref<HTMLTextAreaElement | null>(null)
const { isStreaming, error, onChunk, startStream, stopStream } = useStream()

const assistantMsgId = ref<string>('')
let chunkBuffer = ''
let thinkingBuffer = ''
let referenceBuffer: NonNullable<ChatMessage['references']> = []
let msgCounter = 0

const genId = () => `msg_${Date.now()}_${++msgCounter}`

const hasMessages = computed(() => messages.value.length > 0)

const scrollToBottom = (force = true) => {
  nextTick(() => {
    if (!scrollContainer.value) return
    scrollContainer.value.scrollTop = scrollContainer.value.scrollHeight
  })
}

function simpleMarkdown(text: string): string {
  if (!text) return ''
  let html = text
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
  html = html.replace(/^### (.*?)$/gm, '<h4>$1</h4>')
  html = html.replace(/^## (.*?)$/gm, '<h3>$1</h3>')
  html = html.replace(/^# (.*?)$/gm, '<h2>$1</h2>')
  html = html.replace(/\*\*(.+?)\*\*/g, '<strong>$1</strong>')
  html = html.replace(/`([^`]+)`/g, '<code>$1</code>')
  html = html.replace(/\n/g, '<br>')
  return sanitizeMarkdownHTML(html)
}

const ensureSession = async (): Promise<string> => {
  if (sessionId.value) return sessionId.value
  try {
    const res: any = await createSessions({ title: '新对话' })
    if (res?.data?.id) {
      sessionId.value = res.data.id
      return sessionId.value
    }
    if (res?.data?.session_id) {
      sessionId.value = res.data.session_id
      return sessionId.value
    }
  } catch (e) {
    console.error('[workspace chat] create session failed', e)
  }
  // fallback to temp id
  sessionId.value = 'ws_' + Date.now()
  return sessionId.value
}

const updateStreamingAssistant = () => {
  const assistant = messages.value.find(m => m.id === assistantMsgId.value)
  if (!assistant) return
  assistant.content = chunkBuffer
  assistant.thinking = thinkingBuffer
    assistant.references = referenceBuffer.length ? [...referenceBuffer] : assistant.references
}

const handleChunk = (data: any) => {
  if (!data) return
  const rt = data.response_type || data.type
  const content = data.content != null ? String(data.content) : ''

  if (rt === 'references') {
    const refs = data.knowledge_references || data.data?.references || data.data?.knowledge_references
    if (Array.isArray(refs)) {
      referenceBuffer = refs.map((r: any, i: number) => ({
        id: r.id || r.knowledge_id || String(i + 1),
        title: r.title || r.name || r.doc_title || `来源 ${i + 1}`,
        excerpt: r.excerpt || r.content || r.snippet,
        url: r.url,
      }))
    }
    return
  }

  if (rt === 'thinking') {
    if (content) thinkingBuffer += content
    const assistant = messages.value.find(m => m.id === assistantMsgId.value)
    if (assistant) assistant.thinkingActive = !data.done
    updateStreamingAssistant()
    scrollToBottom()
    return
  }

  if (rt === 'error') {
    const assistant = messages.value.find(m => m.id === assistantMsgId.value)
    if (assistant) {
      assistant.error = content || data.data?.message || '生成失败'
      assistant.isStreaming = false
      assistant.isCompleted = true
    }
    isSending.value = false
    return
  }

  if (rt === 'answer' || !rt || rt === 'message') {
    if (content) chunkBuffer += content
    updateStreamingAssistant()
    scrollToBottom()
    if (data.done) {
      finalizeAssistant()
    }
    return
  }

  if (rt === 'complete' || rt === 'stop') {
    finalizeAssistant()
  }
}

const finalizeAssistant = () => {
  const assistant = messages.value.find(m => m.id === assistantMsgId.value)
  if (assistant) {
    assistant.isStreaming = false
    assistant.isCompleted = true
    assistant.thinkingActive = false
    if (referenceBuffer.length && !assistant.references?.length) {
      assistant.references = [...referenceBuffer]
    }
  }
  chunkBuffer = ''
  thinkingBuffer = ''
  referenceBuffer = []
  assistantMsgId.value = ''
  isSending.value = false
  scrollToBottom()
}

const handleKeydown = (e: KeyboardEvent) => {
  if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
    e.preventDefault()
    sendMessage()
  }
}

const sendMessage = async () => {
  const text = inputText.value.trim()
  if (!text || isSending.value) return

  stopStream()
  const sid = await ensureSession()

  messages.value.push({
    id: genId(),
    role: 'user',
    content: text,
    createdAt: Date.now(),
  })

  chunkBuffer = ''
  thinkingBuffer = ''
  referenceBuffer = []

  const aId = genId()
  assistantMsgId.value = aId
  messages.value.push({
    id: aId,
    role: 'assistant',
    content: '',
    thinking: '',
    thinkingActive: true,
    references: [],
    isStreaming: true,
    isCompleted: false,
    createdAt: Date.now(),
  })

  inputText.value = ''
  isSending.value = true
  nextTick(() => {
    textareaRef.value && autoResize(textareaRef.value)
    scrollToBottom()
  })

  const kbIds = props.knowledgeBaseIds && props.knowledgeBaseIds.length > 0
    ? props.knowledgeBaseIds
    : undefined

  try {
    await startStream({
      session_id: sid,
      query: text,
      knowledge_base_ids: kbIds,
      agent_enabled: true,
      method: 'POST',
      url: '/api/v1/agent-chat',
    })
  } catch (e: any) {
    MessagePlugin.error(e?.message || '发送失败')
    isSending.value = false
  }
}

const handleStop = () => {
  stopStream()
  finalizeAssistant()
}

onChunk(handleChunk)

watch(error, (err) => {
  if (!err) return
  MessagePlugin.error(err)
  isSending.value = false
  const assistant = messages.value.find(m => m.id === assistantMsgId.value)
  if (assistant) {
    assistant.error = err
    assistant.isStreaming = false
    assistant.isCompleted = true
  }
})

const autoResize = (el: HTMLTextAreaElement) => {
  el.style.height = 'auto'
  el.style.height = Math.min(el.scrollHeight, 160) + 'px'
}

const onInput = () => {
  if (textareaRef.value) autoResize(textareaRef.value)
}

onMounted(() => {
  nextTick(() => {
    if (textareaRef.value) autoResize(textareaRef.value)
  })
})

onUnmounted(() => {
  stopStream()
})
</script>

<template>
  <div class="workspace-chat">
    <div ref="scrollContainer" class="chat-scroll">
      <div v-if="!hasMessages" class="chat-empty">
        <div class="empty-icon-wrap">
          <span class="empty-icon-text">XW</span>
        </div>
        <h2 class="empty-title">有什么可以帮您的？</h2>
        <p class="empty-desc">
          基于所选知识库进行问答，支持混合检索、引用溯源和思维链可视化
        </p>
        <div class="sample-questions">
          <button
            v-for="q in ['帮我总结一下 XinWiki 的核心功能', '什么是混合检索？', '如何创建一个知识库页面？']"
            :key="q"
            class="sample-q"
            @click="inputText = q; nextTick(() => textareaRef?.focus())"
          >{{ q }}</button>
        </div>
      </div>

      <div v-else class="message-list">
        <div v-for="msg in messages" :key="msg.id" class="message-row" :class="msg.role">
          <div class="message-bubble" :class="{ streaming: msg.isStreaming, error: !!msg.error }">
            <div v-if="msg.role === 'assistant' && (msg.thinking || msg.thinkingActive)" class="thinking-block">
              <details :open="msg.thinkingActive" class="thinking-details">
                <summary class="thinking-summary">
                  <span class="thinking-dot" :class="{ active: msg.thinkingActive }" />
                  {{ msg.thinkingActive ? '思考中...' : '已深度思考' }}
                </summary>
                <div class="thinking-content" v-html="simpleMarkdown(msg.thinking || '')" />
              </details>
            </div>
            <div v-if="msg.error" class="error-text">{{ msg.error }}</div>
            <div v-else-if="msg.content || msg.isStreaming" class="msg-content markdown-body" v-html="simpleMarkdown(msg.content || '')" />
            <span v-else-if="msg.isStreaming" class="streaming-cursor">▍</span>

            <div v-if="msg.references && msg.references.length > 0" class="references-block">
              <div class="ref-title">引用来源 ({{ msg.references.length }})</div>
              <div class="ref-list">
                <div v-for="(ref, i) in msg.references" :key="ref.id + i" class="ref-item">
                  <span class="ref-num">{{ i + 1 }}</span>
                  <span class="ref-text">{{ ref.title }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <div class="chat-input-wrap">
      <div class="input-box">
        <textarea
          ref="textareaRef"
          v-model="inputText"
          @keydown="handleKeydown"
          @input="onInput"
          placeholder="输入你的问题，⌘Enter 发送"
          class="chat-textarea"
          rows="1"
          :disabled="isSending"
        />
        <button
          class="send-btn"
          :class="{ sending: isSending }"
          :disabled="!inputText.trim() && !isSending"
          @click="isSending ? handleStop() : sendMessage()"
        >
          <StopIcon v-if="isSending" />
          <SendIcon v-else />
        </button>
      </div>
      <div class="input-hint">XinWiki 可能会出错，请核实重要信息</div>
    </div>
  </div>
</template>

<style lang="less" scoped>
.workspace-chat {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

.chat-scroll {
  flex: 1;
  overflow-y: auto;
  overflow-x: hidden;
  padding: 8px 0;
  min-height: 0;

  &::-webkit-scrollbar { width: 6px; }
  &::-webkit-scrollbar-track { background: transparent; }
  &::-webkit-scrollbar-thumb {
    background: rgba(0, 0, 0, 0.12);
    border-radius: 3px;
    &:hover { background: rgba(0, 0, 0, 0.22); }
  }
}

.chat-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  text-align: center;
  padding: 80px 24px 40px;
}

.empty-icon-wrap {
  width: 64px;
  height: 64px;
  border-radius: 50%;
  background: linear-gradient(135deg, #007aff 0%, #5856d6 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 24px;
  box-shadow: 0 8px 24px rgba(0, 122, 255, 0.25);
}
.empty-icon-text {
  color: white;
  font-size: 22px;
  font-weight: 700;
  letter-spacing: 0.02em;
}

.empty-title {
  font-size: 26px;
  font-weight: 700;
  color: #1d1d1f;
  margin: 0 0 8px 0;
  letter-spacing: -0.02em;
}
.empty-desc {
  font-size: 15px;
  color: #86868b;
  margin: 0 0 32px 0;
  max-width: 500px;
  line-height: 1.6;
}

.sample-questions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  justify-content: center;
  max-width: 640px;
}
.sample-q {
  padding: 8px 14px;
  background: rgba(0, 122, 255, 0.06);
  border: 1px solid rgba(0, 122, 255, 0.15);
  border-radius: 16px;
  color: #007aff;
  font-size: 13px;
  cursor: pointer;
  transition: all 0.15s ease;

  &:hover {
    background: rgba(0, 122, 255, 0.12);
    transform: translateY(-1px);
  }
}

.message-list {
  display: flex;
  flex-direction: column;
  gap: 20px;
  padding: 12px 8px;
}

.message-row {
  display: flex;
  width: 100%;

  &.user {
    justify-content: flex-end;
    .message-bubble {
      background: linear-gradient(135deg, #007aff 0%, #5856d6 100%);
      color: white;
      border-bottom-right-radius: 6px;
      max-width: 75%;
    }
    .msg-content { color: white; }
    .msg-content :deep(code) {
      background: rgba(255, 255, 255, 0.2);
      color: white;
    }
  }

  &.assistant {
    justify-content: flex-start;
    .message-bubble {
      background: #fff;
      border: 1px solid rgba(0, 0, 0, 0.06);
      border-bottom-left-radius: 6px;
      max-width: 85%;
    }
  }
}

.message-bubble {
  padding: 12px 16px;
  border-radius: 14px;
  font-size: 15px;
  line-height: 1.65;
  word-break: break-word;
  box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);

  &.streaming {
    .streaming-cursor {
      display: inline-block;
      animation: blink 1s infinite;
      color: #007aff;
      font-weight: 700;
    }
  }

  &.error {
    border-color: rgba(255, 59, 48, 0.3);
    background: rgba(255, 59, 48, 0.04);
  }
}

@keyframes blink {
  0%, 50% { opacity: 1; }
  51%, 100% { opacity: 0; }
}

.error-text {
  color: #ff3b30;
  font-size: 14px;
}

.thinking-block {
  margin-bottom: 12px;
  padding-bottom: 12px;
  border-bottom: 1px dashed rgba(0, 0, 0, 0.08);
}
.thinking-details {
  font-size: 13px;
}
.thinking-summary {
  display: flex;
  align-items: center;
  gap: 8px;
  cursor: pointer;
  color: #86868b;
  list-style: none;
  user-select: none;
  &::-webkit-details-marker { display: none; }
}
.thinking-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #c7c7cc;
  &.active {
    background: #007aff;
    animation: pulse 1.2s ease-in-out infinite;
  }
}
@keyframes pulse {
  0%, 100% { opacity: 0.4; transform: scale(0.9); }
  50% { opacity: 1; transform: scale(1.1); }
}
.thinking-content {
  margin-top: 8px;
  padding: 10px 12px;
  background: rgba(0, 0, 0, 0.02);
  border-radius: 8px;
  color: #6c6c70;
  font-size: 13px;
  line-height: 1.6;
}

.msg-content {
  color: #1d1d1f;
  :deep(h2), :deep(h3), :deep(h4) {
    margin: 12px 0 8px;
    font-weight: 600;
  }
  :deep(code) {
    background: #f5f5f7;
    padding: 2px 6px;
    border-radius: 4px;
    font-family: -apple-system, 'SF Mono', Monaco, monospace;
    font-size: 0.88em;
  }
  :deep(strong) { font-weight: 600; }
}

.references-block {
  margin-top: 12px;
  padding-top: 12px;
  border-top: 1px solid rgba(0, 0, 0, 0.06);
}
.ref-title {
  font-size: 12px;
  font-weight: 600;
  color: #86868b;
  text-transform: uppercase;
  letter-spacing: 0.04em;
  margin-bottom: 8px;
}
.ref-list {
  display: flex;
  flex-direction: column;
  gap: 4px;
}
.ref-item {
  display: flex;
  align-items: center;
  gap: 8px;
  font-size: 13px;
  color: #007aff;
  padding: 4px 0;
  cursor: pointer;
  &:hover { text-decoration: underline; }
}
.ref-num {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 18px;
  height: 18px;
  background: rgba(0, 122, 255, 0.1);
  border-radius: 4px;
  font-size: 11px;
  font-weight: 600;
  flex-shrink: 0;
}
.ref-text {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.chat-input-wrap {
  flex-shrink: 0;
  padding: 16px 8px 8px;
  max-width: 780px;
  width: 100%;
  margin: 0 auto;
}
.input-box {
  position: relative;
  display: flex;
  align-items: flex-end;
  background: white;
  border: 1px solid rgba(0, 0, 0, 0.1);
  border-radius: 16px;
  padding: 10px 52px 10px 16px;
  transition: all 0.2s ease;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.05);

  &:focus-within {
    border-color: #007aff;
    box-shadow: 0 0 0 4px rgba(0, 122, 255, 0.1);
  }
}
.chat-textarea {
  flex: 1;
  border: none;
  outline: none;
  resize: none;
  font-size: 15px;
  line-height: 1.5;
  font-family: inherit;
  background: transparent;
  color: #1d1d1f;
  max-height: 160px;
  padding: 4px 0;

  &::placeholder { color: #86868b; }
  &:disabled { opacity: 0.6; }
}
.send-btn {
  position: absolute;
  right: 10px;
  bottom: 10px;
  width: 32px;
  height: 32px;
  border: none;
  border-radius: 10px;
  background: linear-gradient(135deg, #007aff 0%, #5856d6 100%);
  color: white;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: all 0.15s ease;

  &:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
  &:not(:disabled):hover { transform: scale(1.05); }
  &.sending {
    background: #ff3b30;
    animation: pulse 1.5s ease-in-out infinite;
  }
}
.input-hint {
  text-align: center;
  font-size: 11px;
  color: #86868b;
  margin-top: 8px;
}

@media (max-width: 768px) {
  .message-row.user .message-bubble,
  .message-row.assistant .message-bubble {
    max-width: 90%;
  }
}
</style>
