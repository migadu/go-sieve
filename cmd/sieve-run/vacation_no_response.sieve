require ["vacation"];

# No vacation autoresponse should be sent to our own addresses
vacation :addresses ["sender@example.com", "me@example.com"] 
         "I'm on vacation until next week.";
