package repo

const (
	// Number of hours after purchase before a dispute may no longer be opened
	DisputeOptionTimeoutHours int = 45 * 24
	// Number of hours after dispute begins before it is resolved automatically
	DisputeTotalDurationHours int = 45 * 24

	NotifierTypeBuyerDisputeTimeout           NotificationType = "buyerDisputeTimeout"
	NotifierTypeBuyerDisputeExpiry            NotificationType = "buyerDisputeExpiry"
	NotifierTypeChatMessage                   NotificationType = "chatMessage"
	NotifierTypeChatRead                      NotificationType = "chatRead"
	NotifierTypeChatTyping                    NotificationType = "chatTyping"
	NotifierTypeCompletionNotification        NotificationType = "orderComplete"
	NotifierTypeDisputeAcceptedNotification   NotificationType = "disputeAccepted"
	NotifierTypeDisputeCloseNotification      NotificationType = "disputeClose"
	NotifierTypeDisputeOpenNotification       NotificationType = "disputeOpen"
	NotifierTypeDisputeUpdateNotification     NotificationType = "disputeUpdate"
	NotifierTypeFindModeratorResponse         NotificationType = "findModeratorResponse"
	NotifierTypeFollowNotification            NotificationType = "follow"
	NotifierTypeFulfillmentNotification       NotificationType = "fulfillment"
	NotifierTypeIncomingTransaction           NotificationType = "incomingTransaction"
	NotifierTypeModeratorAddNotification      NotificationType = "moderatorAdd"
	NotifierTypeModeratorDisputeExpiry        NotificationType = "moderatorDisputeExpiry"
	NotifierTypeModeratorRemoveNotification   NotificationType = "moderatorRemove"
	NotifierTypeOrderCancelNotification       NotificationType = "cancel"
	NotifierTypeOrderConfirmationNotification NotificationType = "orderConfirmation"
	NotifierTypeOrderDeclinedNotification     NotificationType = "orderDeclined"
	NotifierTypeOrderNewNotification          NotificationType = "order"
	NotifierTypePaymentNotification           NotificationType = "payment"
	NotifierTypePremarshalledNotifier         NotificationType = "premarshalledNotifier"
	NotifierTypeProcessingErrorNotification   NotificationType = "processingError"
	NotifierTypeRefundNotification            NotificationType = "refund"
	NotifierTypeStatusUpdateNotification      NotificationType = "statusUpdate"
	NotifierTypeTestNotification              NotificationType = "testNotification"
	NotifierTypeUnfollowNotification          NotificationType = "unfollow"
	NotifierTypeVendorDisputeTimeout          NotificationType = "vendorDisputeTimeout"
	NotifierTypeVendorFinalizedPayment        NotificationType = "vendorFinalizedPayment"
)

type NotificationType string

func (t NotificationType) String() string { return string(t) }
