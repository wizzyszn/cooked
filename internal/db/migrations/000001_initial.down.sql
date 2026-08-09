-- Drop in dependency order: recipes → delicacies → users

DROP TABLE IF EXISTS recipes CASCADE;
DROP TABLE IF EXISTS delicacies CASCADE;
DROP TABLE IF EXISTS users CASCADE;
