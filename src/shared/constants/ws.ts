/**
 * every value is its own literal type.
 * 
 * the server sends a tagged union down a single socket
 * ! what is a tag union:
 * Tagged unions are data structures that can hold a value of several different types, 
 * with a tag indicating which type is currently in use, while enums are a specific type that defines a set of named values. 
 * The main difference is that tagged unions can store different types of data, 
 * whereas enums are typically used to represent a fixed set of related constants.
 *   
 *   realtime.TypePing / TypePong       (platform/realtime/frame.go)
 *   chat.Frame*                        (modules/chat/model.go)
 *   notifications.FrameNotificationNew (modules/notifications/model.go)
 */
export const WS_PATH = '/ws' as const

export const WsFrame = {
  DmMessage: 'dm.message',
  DmTyping: 'dm.typing',
  GroupMessage: 'group.message',
  /** groups.FrameGroupPost — a post appearing in a group the reader belongs to */
  GroupPost: 'group.post',
  NotificationNew: 'notification.new',
  /** notifications.FrameNotificationGone — notifications that stopped being true */
  NotificationGone: 'notification.gone',
  Ping: 'ping',
  Pong: 'pong',
} as const

export type WsFrameType = (typeof WsFrame)[keyof typeof WsFrame]
