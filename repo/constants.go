package repo

const (
	NotifierTypeBuyerDisputeTimeout           NotificationType = "buyerDisputeTimeout"
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

	// SaleAging
	NotifierTypeSaleAgedFourtyFiveDays NotificationType = "saleAgedFourtyFiveDays"
	// End Notification Types
)

type NotificationType string

func (t NotificationType) String() string { return string(t) }
