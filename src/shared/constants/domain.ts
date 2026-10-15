/**
 * the values both sides have to agree on, where a typo is a silent bug rather than a loud one.
 * ! these strings are written by go handlers and read by tsx components. keep them in one place
 * ! and a rename is caught by the compiler, duplicate them and it is caught by a user staring
 * ! at an empty feed
 *
 * mirrors:
 *   posts.Privacy*            (modules/posts/models.go)
 *   groups.Status* / Role*    (modules/groups/model.go)
 *   events.*                  (platform/events/events.go)
 */

/** posts.PrivacyPublic / PrivacyAlmostPrivate / PrivacyPrivate */
export const Privacy = {
  Public: 'public',
  AlmostPrivate: 'almost_private',
  Private: 'private',
} as const
export type Privacy = (typeof Privacy)[keyof typeof Privacy]

/**
 * follows.status, as stored in the follows table.
 * ? the empty string is not a stored value, it is what users.Profile.follow_status carries
 * ? when there is no follows row at all
 */
export const FollowStatus = {
  None: '',
  Pending: 'pending',
  Accepted: 'accepted',
} as const
export type FollowStatus = (typeof FollowStatus)[keyof typeof FollowStatus]

/** groups.StatusInvited / StatusRequested / StatusMember, plus the no-row case */
export const MemberStatus = {
  None: '',
  Invited: 'invited',
  Requested: 'requested',
  Member: 'member',
} as const
export type MemberStatus = (typeof MemberStatus)[keyof typeof MemberStatus]

/** groups.RoleCreator / RoleMember */
export const MemberRole = {
  None: '',
  Creator: 'creator',
  Member: 'member',
} as const
export type MemberRole = (typeof MemberRole)[keyof typeof MemberRole]

/** groups.join_policy, the CHECK in 000007_create_groups_table.up.sql */
export const JoinPolicy = {
  Request: 'request',
  Open: 'open',
} as const
export type JoinPolicy = (typeof JoinPolicy)[keyof typeof JoinPolicy]

/** event_responses.response */
export const EventResponse = {
  None: '',
  Going: 'going',
  NotGoing: 'not_going',
} as const
export type EventResponse = (typeof EventResponse)[keyof typeof EventResponse]

/**
 * the four kinds the bus publishes, which are also the notification type stored in the db.
 * ! events.go says it outright: the strings are the same on both sides so the subscriber
 * ! inserts without a translation step, and they line up with the CHECK on notifications.type
 */
export const NotificationType = {
  FollowRequest: 'follow_request',
  GroupInvite: 'group_invite',
  GroupJoinRequest: 'group_join_request',
  EventCreated: 'event_created',
} as const
export type NotificationType = (typeof NotificationType)[keyof typeof NotificationType]

/** the labels the mockup prints for each kind, at line 3133. kept verbatim */
export const NOTIFICATION_LABEL: Record<NotificationType, string> = {
  follow_request: 'follow request',
  group_invite: 'group invite',
  group_join_request: 'join request',
  event_created: 'event created',
}

/** the three types storage.Save will accept, sniffed from the bytes rather than the extension */
export const ACCEPTED_IMAGE_TYPES = 'image/jpeg,image/png,image/gif'
