import { combineReducers } from 'redux';

import environmentsReducer from 'reducers/environments-reducer';

export default combineReducers({
  environments: environmentsReducer
});