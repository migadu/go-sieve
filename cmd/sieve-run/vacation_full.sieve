require ["vacation"];

# Vacation autoresponse with all parameters
vacation :days 14 
         :subject "Out of Office" 
         :from "me@example.com" 
         :addresses ["me@example.com", "me2@example.com"] 
         :mime 
         :handle "vacation-001" 
         "I'm on vacation until June 1st. For urgent matters, please contact my colleague at colleague@example.com.";
