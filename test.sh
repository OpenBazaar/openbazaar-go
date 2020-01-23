#!/bin/bash
for i in {1..10}; do
  #time curl -X POST 
  curl -d '{"address":"","addressNotes":"","alternateContactInfo":"","city":"","countryCode":"","items":[{"listingHash":"Qme3QN5LjZsQ82hdJtUjnpsWdDJi9cccuav5QMWHeDkqiH","options":[],"shipping":{"name":""},"memo":"","coupons":[],"bigQuantity":"5"}],"moderator":"","paymentCoin":"LTC","postalCode":"","shipTo":"","state":""}' \
    '0.0.0.0:4002/ob/purchase'
done
