import { combineReducers } from 'redux';

import environmentsReducer from 'reducers/environments-reducer';
import databasesReducer from 'reducers/databases-reducer';
import sessionReducer from 'reducers/session-reducer';

export default combineReducers({
  environments: environmentsReducer,
  databases: databasesReducer,
  session: sessionReducer
});