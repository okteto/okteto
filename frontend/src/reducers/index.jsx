import { combineReducers } from 'redux';

import environmentsReducer from 'reducers/environments-reducer';
import sessionReducer from 'reducers/session-reducer';

export default combineReducers({
  environments: environmentsReducer,
  session: sessionReducer
});