## Architecture


### Processing responder and comment actions

A database trigger will ensure that `INSERT` or `UPDATE` to the `responder` table will be inserted into a `responder_queue` table. A worker process will be watching this table for inserts.
Each insert would trigger a reconcilation loop, which will ensure that the state remains consistent.

Similarly, comments for an incident will be handled. There is a possibility that an incident may have mulitple responders, so adding a comment would update all the responder's referenced via the `incident_id`.  
