-- the follow_request notifications left behind before a cancel started retracting its own
-- notice. each one points at a follows row that was deleted the moment the sender withdrew,
-- so it offers an accept that answers 404 and a decline that answers 404.
--! unread only. a READ follow_request is history the reader has already dealt with, and the
--! notices channel is documented as keeping resolved items — the dead ones are exactly the
--! ones that were never resolved and never could be
--! this is data cleanup and not a schema change, which is why it deletes nothing that the
--! application does not already refuse to show: the `live` predicate in
--! notifications/store.go hides precisely this set from the list and from the unread count
--? one-off. from here on the rows never accumulate, because cancelling a pending request
--? publishes events.FollowRequestCancelled and the subscriber deletes the row at the source
DELETE FROM notifications
 WHERE type = 'follow_request'
   AND read_at IS NULL
   AND NOT EXISTS (
         SELECT 1 FROM follows f
          WHERE f.id = notifications.entity_id
            AND f.status = 'pending'
       );
